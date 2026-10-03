package research

import (
	"context"
	"errors"
	"fmt"
	"strings"

	firecrawl "github.com/firecrawl/firecrawl/apps/go-sdk"
	"github.com/firecrawl/firecrawl/apps/go-sdk/option"
)

// SearchResult is one discovery hit, normalized across providers.
type SearchResult struct {
	URL     string
	Title   string
	Snippet string
}

// SearchUsage is the provider's credit/concurrency reading around a search.
type SearchUsage struct {
	CreditsUsed    int
	CreditsRemain  int
	Concurrency    int
	MaxConcurrency int
}

// SearchProvider discovers sources for a query. The CLI talks to this
// interface, never to a concrete SDK, so tests use a fake and the core stays
// free of a hard dependency on any one provider.
type SearchProvider interface {
	// Search returns at most limit results for query.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	// CreditUsage returns the remaining credits, or -1 when unknown.
	CreditUsage(ctx context.Context) (int, error)
	// Name identifies the provider in the manifest.
	Name() string
}

// ErrMissingAPIKey is returned when no Firecrawl API key is configured.
var ErrMissingAPIKey = errors.New("FIRECRAWL_API_KEY is not set (put it in the shell or in the project .env)")

// FirecrawlProvider is the SearchProvider backed by the official Firecrawl Go
// SDK. It uses only Search for discovery (never Scrape/Crawl/Map/Agent).
type FirecrawlProvider struct {
	client *firecrawl.Client
}

// NewFirecrawlProvider builds a provider from an explicit key and base URL. When
// apiURL is empty the SDK default endpoint is used; when apiKey is empty the
// SDK reads FIRECRAWL_API_KEY itself, and a missing key surfaces on first use.
func NewFirecrawlProvider(apiKey, apiURL string) (*FirecrawlProvider, error) {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if apiURL != "" {
		opts = append(opts, option.WithAPIURL(apiURL))
	}
	client, err := firecrawl.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("firecrawl client: %w", err)
	}
	return &FirecrawlProvider{client: client}, nil
}

// Name identifies the provider in the run manifest.
func (p *FirecrawlProvider) Name() string { return "firecrawl" }

// Search runs a web search and normalizes the results.
func (p *FirecrawlProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	opts := &firecrawl.SearchOptions{}
	if limit > 0 {
		opts.Limit = firecrawl.Int(limit)
	}
	data, err := p.client.Search(ctx, query, opts)
	if err != nil {
		return nil, mapFirecrawlError(err)
	}
	if data == nil {
		return nil, nil
	}
	out := make([]SearchResult, 0, len(data.Web))
	for _, item := range data.Web {
		out = append(out, SearchResult{
			URL:     stringField(item, "url"),
			Title:   stringField(item, "title"),
			Snippet: stringField(item, "description"),
		})
	}
	return out, nil
}

// CreditUsage returns the remaining credits, or -1 when the reading fails.
func (p *FirecrawlProvider) CreditUsage(ctx context.Context) (int, error) {
	usage, err := p.client.GetCreditUsage(ctx)
	if err != nil {
		return -1, mapFirecrawlError(err)
	}
	if usage == nil {
		return -1, nil
	}
	return usage.RemainingCredits, nil
}

// Concurrency returns the active/max concurrency, or -1s when unknown.
func (p *FirecrawlProvider) Concurrency(ctx context.Context) (int, int) {
	check, err := p.client.GetConcurrency(ctx)
	if err != nil || check == nil {
		return -1, -1
	}
	return check.Concurrency, check.MaxConcurrency
}

// mapFirecrawlError turns the SDK's typed errors into one clear message.
func mapFirecrawlError(err error) error {
	var auth *firecrawl.AuthenticationError
	var rate *firecrawl.RateLimitError
	var timeout *firecrawl.JobTimeoutError
	switch {
	case errors.As(err, &auth):
		return fmt.Errorf("firecrawl rejected the API key (HTTP 401): %s", auth.Message)
	case errors.As(err, &rate):
		return fmt.Errorf("firecrawl rate limit reached (HTTP 429): %s", rate.Message)
	case errors.As(err, &timeout):
		return fmt.Errorf("firecrawl job %s timed out after %ds", timeout.JobID, timeout.TimeoutSeconds)
	default:
		return err
	}
}

// stringField reads a string field from a loosely-typed SDK result map.
func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
