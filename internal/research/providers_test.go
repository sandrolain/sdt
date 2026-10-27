package research

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// env returns a getenv that knows only the variables a test declares.
func env(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

func TestProviderSetSkipsWhatItCannotUse(t *testing.T) {
	set, skipped, err := NewProviderSet(DefaultProviders(), env(map[string]string{"TAVILY_API_KEY": "t"}))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := strings.Join(set.Names(), ","); got != "tavily" {
		t.Fatalf("usable = %q, want only tavily (the others lack a key or an endpoint)", got)
	}
	if len(skipped) != 3 {
		t.Fatalf("skipped = %+v, want the three unusable entries", skipped)
	}
	for _, want := range []string{"FIRECRAWL_API_KEY is not set", "EXA_API_KEY is not set", "no endpoint declared"} {
		found := false
		for _, s := range skipped {
			if strings.Contains(s.Skipped, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("skipped reasons miss %q: %+v", want, skipped)
		}
	}
}

func TestProviderSetUsesTheDeclaredOrder(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "quota exhausted", http.StatusTooManyRequests)
	}))
	defer failing.Close()
	answering := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"url": "https://example.com/a", "title": "A", "text": "from exa"}},
		})
	}))
	defer answering.Close()

	set, _, err := NewProviderSet([]ProviderConfig{
		{Provider: "tavily", Endpoint: failing.URL, Key: "k"},
		{Provider: "exa", Endpoint: answering.URL, Key: "k"},
	}, env(nil))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	results, attempts, err := set.Search(context.Background(), "q", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Snippet != "from exa" {
		t.Fatalf("results = %+v, want the second provider's answer", results)
	}
	if len(attempts) != 2 || attempts[0].OK || !attempts[1].OK {
		t.Fatalf("attempts = %+v, want a failure then a success", attempts)
	}
	if !strings.Contains(attempts[0].Error, "quota exhausted") {
		t.Fatalf("the failed attempt must carry the provider's sentence: %+v", attempts[0])
	}
}

func TestProviderSetNothingUsableNamesEveryReason(t *testing.T) {
	set, _, err := NewProviderSet(DefaultProviders(), env(nil))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_, attempts, err := set.Search(context.Background(), "q", 3)
	if err == nil || !strings.Contains(err.Error(), "no research search provider") {
		t.Fatalf("error = %v, want ErrNoProvider", err)
	}
	if len(attempts) != 4 {
		t.Fatalf("attempts = %+v, want every declared entry", attempts)
	}
	for _, name := range []string{"tavily", "firecrawl", "exa", "searxng"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the sentence must name %q: %v", name, err)
		}
	}
}

func TestProviderSetRejectsAnUnknownProvider(t *testing.T) {
	if _, _, err := NewProviderSet([]ProviderConfig{{Provider: "bing"}}, env(nil)); err == nil {
		t.Fatal("an unknown provider must be a configuration error")
	}
}

func TestProviderKeyCanComeFromAFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tavily.key")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	set, skipped, err := NewProviderSet([]ProviderConfig{{Provider: "tavily", KeyFile: path}}, env(nil))
	if err != nil || len(skipped) != 0 {
		t.Fatalf("build: %v skipped=%+v", err, skipped)
	}
	if got := set.Names(); len(got) != 1 || got[0] != "tavily" {
		t.Fatalf("usable = %v", got)
	}
}

func TestTavilyRequestShape(t *testing.T) {
	var got map[string]any
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"url": "https://example.com/t", "title": "T", "content": "snippet"}},
		})
	}))
	defer server.Close()

	p := &httpProvider{name: "tavily", endpoint: server.URL, key: "secret", client: server.Client()}
	results, err := p.Search(context.Background(), "vector stores", 4)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if auth != "Bearer secret" {
		t.Fatalf("Authorization = %q, want the bearer token the official client sends", auth)
	}
	if _, inBody := got["api_key"]; inBody {
		t.Fatalf("the key must not travel in the body: %+v", got)
	}
	if got["query"] != "vector stores" || got["max_results"] != float64(4) || got["search_depth"] != "basic" {
		t.Fatalf("request = %+v", got)
	}
	if len(results) != 1 || results[0].Snippet != "snippet" {
		t.Fatalf("results = %+v", results)
	}
}

func TestExaRequestShape(t *testing.T) {
	var key string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("x-api-key")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"url": "https://example.com/e", "title": "E", "text": "exa text"}},
		})
	}))
	defer server.Close()

	p := &httpProvider{name: "exa", endpoint: server.URL, key: "exa-key", client: server.Client()}
	results, err := p.Search(context.Background(), "q", 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if key != "exa-key" {
		t.Fatalf("x-api-key = %q", key)
	}
	if len(results) != 1 || results[0].Snippet != "exa text" {
		t.Fatalf("results = %+v", results)
	}
}

func TestSearxngRequestShape(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.URL.Query().Get("format") != "json" {
			t.Errorf("path = %q format = %q", r.URL.Path, r.URL.Query().Get("format"))
		}
		query = r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"url": "https://example.com/s", "title": "S", "content": "searx snippet"}},
		})
	}))
	defer server.Close()

	p := &httpProvider{name: "searxng", endpoint: server.URL, client: server.Client()}
	results, err := p.Search(context.Background(), "go vector store", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if query != "go vector store" || len(results) != 1 || results[0].Snippet != "searx snippet" {
		t.Fatalf("query = %q results = %+v", query, results)
	}
}

func TestHTTPProviderErrorCarriesTheBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()
	p := &httpProvider{name: "exa", endpoint: server.URL, key: "k", client: server.Client()}
	_, err := p.Search(context.Background(), "q", 1)
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("error = %v, want the status and the provider's sentence", err)
	}
}
