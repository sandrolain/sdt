package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ProviderConfig is one entry of the project's `research:` array: a provider, an optional
// endpoint override, and one credential reference. The array's order is the fallback order.
type ProviderConfig struct {
	Provider string `json:"provider" yaml:"provider"`
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	// Key is a literal credential. KeyEnv names a variable, KeyFile a file holding it.
	// At most one is used; a literal wins when several are declared.
	Key     string `json:"key,omitempty" yaml:"key,omitempty"`
	KeyEnv  string `json:"key_env,omitempty" yaml:"key_env,omitempty"`
	KeyFile string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// providerDefaults is where each provider lives when the entry declares nothing: the
// endpoint, and the variable a key is read from. SearXNG has neither — it is a user
// instance, so it is available only when an endpoint is declared.
type providerDefaults struct {
	endpoint string
	keyEnv   string
	needsKey bool
}

// Provider names, in one spelling: the config key, the manifest value and the error
// sentences all read the same constant.
const (
	ProviderTavily    = "tavily"
	ProviderFirecrawl = "firecrawl"
	ProviderExa       = "exa"
	ProviderSearxng   = "searxng"
)

var knownProviders = map[string]providerDefaults{
	ProviderTavily:    {endpoint: "https://api.tavily.com", keyEnv: "TAVILY_API_KEY", needsKey: true},
	ProviderFirecrawl: {endpoint: "", keyEnv: "FIRECRAWL_API_KEY", needsKey: true},
	ProviderExa:       {endpoint: "https://api.exa.ai", keyEnv: "EXA_API_KEY", needsKey: true},
	ProviderSearxng:   {endpoint: "", keyEnv: "", needsKey: false},
}

// DefaultProviders is the order a project with no `research:` block runs: Tavily first,
// then Firecrawl, Exa and the self-hosted SearXNG. Each entry is unavailable until its
// key (or, for SearXNG, its endpoint) is set.
func DefaultProviders() []ProviderConfig {
	return []ProviderConfig{
		{Provider: ProviderTavily},
		{Provider: ProviderFirecrawl},
		{Provider: ProviderExa},
		{Provider: ProviderSearxng},
	}
}

// ErrNoProvider is returned when nothing in the declared order is usable, or when every
// usable provider failed. The sentence names each reason.
var ErrNoProvider = errors.New("no research search provider is available")

// Attempt is one step of the walk: who answered, who was skipped and why, who failed.
type Attempt struct {
	Provider string `json:"provider" yaml:"provider"`
	OK       bool   `json:"ok,omitempty" yaml:"ok,omitempty"`
	Skipped  string `json:"skipped,omitempty" yaml:"skipped,omitempty"`
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
}

// ProviderSet walks the declared providers in order. A provider whose credential is not
// resolvable is skipped with the reason kept, not treated as a failure: the walk continues
// to the next one, which is what "a paid service is used only when its key is set" means.
type ProviderSet struct {
	providers []SearchProvider
	skipped   []Attempt
}

// NewProviderSet builds the usable providers from the declared order. A provider the
// build cannot use is recorded in the skipped list and left out; an unknown provider name
// is a configuration error, not a skip.
func NewProviderSet(cfgs []ProviderConfig, getenv func(string) string) (*ProviderSet, []Attempt, error) {
	if len(cfgs) == 0 {
		cfgs = DefaultProviders()
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	set := &ProviderSet{}
	var skipped []Attempt
	for _, cfg := range cfgs {
		name := strings.ToLower(strings.TrimSpace(cfg.Provider))
		defaults, known := knownProviders[name]
		if !known {
			return nil, nil, fmt.Errorf("research: unknown provider %q (known: firecrawl, exa, tavily, searxng)", cfg.Provider)
		}
		key, keyReason := resolveProviderKey(cfg, defaults, getenv)
		if defaults.needsKey && key == "" {
			skipped = append(skipped, Attempt{Provider: name, Skipped: keyReason})
			continue
		}
		endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if endpoint == "" {
			endpoint = defaults.endpoint
		}
		if name == ProviderSearxng && endpoint == "" {
			skipped = append(skipped, Attempt{Provider: name, Skipped: "no endpoint declared (SearXNG is a user instance)"})
			continue
		}
		provider, err := newHTTPProvider(name, endpoint, key)
		if err != nil {
			return nil, nil, err
		}
		set.providers = append(set.providers, provider)
	}
	set.skipped = skipped
	return set, skipped, nil
}

// resolveProviderKey reads the credential from the entry: a literal, then a variable, then
// a file. A missing one yields the sentence the skipped attempt carries.
func resolveProviderKey(cfg ProviderConfig, defaults providerDefaults, getenv func(string) string) (string, string) {
	if key := strings.TrimSpace(cfg.Key); key != "" {
		return key, ""
	}
	if envName := strings.TrimSpace(cfg.KeyEnv); envName != "" {
		if key := strings.TrimSpace(getenv(envName)); key != "" {
			return key, ""
		}
		return "", fmt.Sprintf("%s is not set", envName)
	}
	if file := strings.TrimSpace(cfg.KeyFile); file != "" {
		//#nosec G304 -- the path is the project's own declaration
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Sprintf("%s cannot be read: %v", file, err)
		}
		if key := strings.TrimSpace(string(data)); key != "" {
			return key, ""
		}
		return "", fmt.Sprintf("%s is empty", file)
	}
	if defaults.keyEnv == "" {
		return "", ""
	}
	if key := strings.TrimSpace(getenv(defaults.keyEnv)); key != "" {
		return key, ""
	}
	return "", fmt.Sprintf("%s is not set", defaults.keyEnv)
}

// NewProviderSetFrom builds a set from providers that already exist, in the given order.
// It is the seam a caller with its own list uses — a test, or a future source that is not
// the project file — while the config path stays NewProviderSet.
func NewProviderSetFrom(providers ...SearchProvider) *ProviderSet {
	return &ProviderSet{providers: providers}
}

// Names lists the usable providers, in the order they will be tried.
func (s *ProviderSet) Names() []string {
	out := make([]string, 0, len(s.providers))
	for _, p := range s.providers {
		out = append(out, p.Name())
	}
	return out
}

// Search walks the usable providers in order and returns the first answer. Every step is
// reported: the skipped ones from the declaration, then one attempt per provider tried.
// When nothing is usable, or every usable provider fails, the error names each reason.
func (s *ProviderSet) Search(ctx context.Context, query string, limit int) ([]SearchResult, []Attempt, error) {
	attempts := append([]Attempt(nil), s.skipped...)
	if len(s.providers) == 0 {
		return nil, attempts, fmt.Errorf("%w: %s", ErrNoProvider, attemptSentence(attempts))
	}
	for _, provider := range s.providers {
		results, err := provider.Search(ctx, query, limit)
		if err != nil {
			attempts = append(attempts, Attempt{Provider: provider.Name(), Error: err.Error()})
			continue
		}
		attempts = append(attempts, Attempt{Provider: provider.Name(), OK: true})
		return results, attempts, nil
	}
	return nil, attempts, fmt.Errorf("%w: %s", ErrNoProvider, attemptSentence(attempts))
}

// CreditUsage reports the remaining credits of the first usable provider, which is the one
// a run would spend first; -1 means the provider does not report them.
func (s *ProviderSet) CreditUsage(ctx context.Context) (int, error) {
	if len(s.providers) == 0 {
		return -1, ErrNoProvider
	}
	return s.providers[0].CreditUsage(ctx)
}

// attemptSentence renders every attempt as "name: reason", so one error carries the whole
// walk.
func attemptSentence(attempts []Attempt) string {
	if len(attempts) == 0 {
		return "no provider declared"
	}
	parts := make([]string, 0, len(attempts))
	for _, a := range attempts {
		reason := a.Error
		if reason == "" {
			reason = a.Skipped
		}
		if reason == "" && a.OK {
			reason = "answered"
		}
		parts = append(parts, a.Provider+": "+reason)
	}
	return strings.Join(parts, "; ")
}

// --- the three HTTP providers -----------------------------------------------------------

// httpProvider is one provider reached over its own JSON endpoint. No SDK: the search
// endpoint of each service is a single request and a single shape.
type httpProvider struct {
	name     string
	endpoint string
	key      string
	client   *http.Client
}

func newHTTPProvider(name, endpoint, key string) (SearchProvider, error) {
	if name == ProviderFirecrawl {
		// Firecrawl keeps its SDK: it is the vendor the lane shipped on, and its client
		// already maps the error shapes the receipt knows.
		return NewFirecrawlProvider(key, endpoint)
	}
	return &httpProvider{name: name, endpoint: endpoint, key: key, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (p *httpProvider) Name() string { return p.name }

// CreditUsage is unknown for these three: Tavily reports credits per response, Exa bills
// per request, SearXNG has none. -1 is the seam's "unknown".
func (p *httpProvider) CreditUsage(context.Context) (int, error) { return -1, nil }

func (p *httpProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	switch p.name {
	case ProviderTavily:
		return p.searchTavily(ctx, query, limit)
	case ProviderExa:
		return p.searchExa(ctx, query, limit)
	case ProviderSearxng:
		return p.searchSearxng(ctx, query)
	}
	return nil, fmt.Errorf("%s: no search implementation", p.name)
}

func (p *httpProvider) postJSON(ctx context.Context, path string, headers map[string]string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%s: encode request: %w", p.name, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+path, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("%s: build request: %w", p.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return p.do(req, out)
}

func (p *httpProvider) do(req *http.Request, out any) error {
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // read-only body
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%s: read response: %w", p.name, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d: %s", p.name, resp.StatusCode, snippet(data))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: decode response: %w", p.name, err)
	}
	return nil
}

func snippet(data []byte) string {
	s := strings.Join(strings.Fields(string(data)), " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func (p *httpProvider) searchTavily(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	var body struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	// The official client authenticates with a bearer token and never sends the key in the
	// body; the request is kept to the fields the search helper sends.
	err := p.postJSON(ctx, "/search", map[string]string{"Authorization": "Bearer " + p.key}, map[string]any{
		"query":        query,
		"max_results":  limit,
		"search_depth": "basic",
	}, &body)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(body.Results))
	for _, r := range body.Results {
		out = append(out, SearchResult{URL: r.URL, Title: r.Title, Snippet: r.Content})
	}
	return out, nil
}

func (p *httpProvider) searchExa(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	var body struct {
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Text  string `json:"text"`
		} `json:"results"`
	}
	err := p.postJSON(ctx, "/search", map[string]string{"x-api-key": p.key}, map[string]any{
		"query":      query,
		"numResults": limit,
	}, &body)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(body.Results))
	for _, r := range body.Results {
		out = append(out, SearchResult{URL: r.URL, Title: r.Title, Snippet: r.Text})
	}
	return out, nil
}

func (p *httpProvider) searchSearxng(ctx context.Context, query string) ([]SearchResult, error) {
	endpoint := p.endpoint + "/search?q=" + url.QueryEscape(query) + "&format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("searxng: build request: %w", err)
	}
	var body struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := p.do(req, &body); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(body.Results))
	for _, r := range body.Results {
		out = append(out, SearchResult{URL: r.URL, Title: r.Title, Snippet: r.Content})
	}
	return out, nil
}
