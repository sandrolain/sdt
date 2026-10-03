package research

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// firecrawlTestServer fakes the Firecrawl v2 API far enough for the provider:
// POST /v2/search returns a web result list, GET /v2/team/credit-usage returns
// remaining credits.
func firecrawlTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false, "error": "invalid key",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"web": []map[string]any{
					{"url": "https://example.com/a", "title": "A", "description": "first"},
					{"url": "https://example.com/b", "title": "B", "description": "second"},
				},
			},
		})
	})
	mux.HandleFunc("/v2/team/credit-usage", func(w http.ResponseWriter, r *http.Request) {
		// GetCreditUsage decodes the body directly into CreditUsage (no envelope).
		_ = json.NewEncoder(w).Encode(map[string]any{"remainingCredits": 42, "planCredits": 100})
	})
	mux.HandleFunc("/v2/concurrency-check", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"concurrency": 1, "maxConcurrency": 5})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestFirecrawlProviderSearchNormalizesResults(t *testing.T) {
	ts := firecrawlTestServer(t)
	p, err := NewFirecrawlProvider("test-key", ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Search(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %d, want 2: %+v", len(got), got)
	}
	if got[0].URL != "https://example.com/a" || got[0].Title != "A" || got[0].Snippet != "first" {
		t.Errorf("result 0 = %+v", got[0])
	}
	if p.Name() != "firecrawl" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestFirecrawlProviderCreditUsage(t *testing.T) {
	ts := firecrawlTestServer(t)
	p, err := NewFirecrawlProvider("test-key", ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := p.CreditUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 42 {
		t.Errorf("remaining = %d, want 42", remaining)
	}
}

func TestFirecrawlProviderAuthError(t *testing.T) {
	ts := firecrawlTestServer(t)
	p, err := NewFirecrawlProvider("wrong-key", ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Search(context.Background(), "x", 1)
	if err == nil {
		t.Fatal("expected an auth error")
	}
	if got := err.Error(); !strings.Contains(got, "401") {
		t.Errorf("error %q should mention HTTP 401", got)
	}
}

func TestFirecrawlProviderMissingKeySurfacesOnUse(t *testing.T) {
	// No key: the SDK reads FIRECRAWL_API_KEY; with it unset the first call must
	// fail rather than panic. Point at an unreachable URL so no real request is
	// attempted.
	t.Setenv("FIRECRAWL_API_KEY", "")
	p, err := NewFirecrawlProvider("", "http://127.0.0.1:0")
	if err != nil {
		// Some SDK versions validate the key at construction; that is fine.
		return
	}
	if _, err := p.Search(context.Background(), "x", 1); err == nil {
		t.Fatal("expected an error without a key")
	}
}

func TestMapFirecrawlErrorPassthrough(t *testing.T) {
	base := errors.New("boom")
	if got := mapFirecrawlError(base); !errors.Is(got, base) {
		t.Errorf("a non-SDK error must pass through unchanged, got %v", got)
	}
}
