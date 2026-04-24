package crawler

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

// Page represents a crawled web page.
type Page struct {
	URL     string
	Title   string
	Content string
}

// Options defines crawler configuration.
type Options struct {
	MaxDepth            int
	AllowedDomains      []string
	UserAgent           string
	IgnoreRobotsTxt     bool
	FollowExternalLinks bool
	SinglePage          bool
	RequestTimeout      int
	RequestDelay        int
	ExcludedPaths       []string
	Silent              bool
}

// PageCallback is called when a page is successfully crawled.
type PageCallback func(page Page)

// Crawler handles web crawling operations.
type Crawler struct {
	collector    *colly.Collector
	pages        []Page
	pagesMutex   sync.Mutex
	baseURL      *url.URL
	options      Options
	pageCallback PageCallback
	output       io.Writer
}

// NewCrawler creates a new Crawler instance for the given start URL.
func NewCrawler(startURL string, opts Options) (*Crawler, error) {
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if opts.MaxDepth == 0 {
		opts.MaxDepth = 2
	}

	if opts.UserAgent == "" {
		opts.UserAgent = "sdt/1.0"
	}

	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 30
	}

	allowedDomains := opts.AllowedDomains
	if len(allowedDomains) == 0 && !opts.FollowExternalLinks {
		allowedDomains = []string{parsedURL.Host}
	}

	c := colly.NewCollector(
		colly.MaxDepth(opts.MaxDepth),
		colly.AllowedDomains(allowedDomains...),
		colly.UserAgent(opts.UserAgent),
		colly.Async(true),
	)

	c.SetRequestTimeout(time.Duration(opts.RequestTimeout) * time.Second)

	if opts.RequestDelay > 0 {
		if err := c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Delay:       time.Duration(opts.RequestDelay) * time.Second,
			RandomDelay: time.Duration(opts.RequestDelay/2) * time.Second,
			Parallelism: 2,
		}); err != nil {
			return nil, fmt.Errorf("set rate limit: %w", err)
		}
	} else {
		//nolint:errcheck
		_ = c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: 2,
		})
	}

	if opts.IgnoreRobotsTxt {
		c.IgnoreRobotsTxt = true
	}

	output := io.Writer(os.Stdout)
	if opts.Silent {
		output = io.Discard
	}

	cr := &Crawler{
		collector: c,
		pages:     []Page{},
		baseURL:   parsedURL,
		options:   opts,
		output:    output,
	}

	return cr, nil
}

// OnPage registers a callback invoked for each successfully crawled page.
func (c *Crawler) OnPage(callback PageCallback) {
	c.pageCallback = callback
}

// Start begins the crawling process and blocks until completion.
func (c *Crawler) Start() error {
	c.setupCallbacks()

	if err := c.collector.Visit(c.baseURL.String()); err != nil {
		return fmt.Errorf("start crawling: %w", err)
	}

	c.collector.Wait()

	return nil
}

func (c *Crawler) setupCallbacks() {
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		normalizedURL := normalizeURL(e.Request.URL.String())

		page := Page{
			URL:     normalizedURL,
			Title:   e.ChildText("title"),
			Content: extractMainContent(e),
		}

		c.pagesMutex.Lock()
		c.pages = append(c.pages, page)
		c.pagesMutex.Unlock()

		if c.pageCallback != nil {
			c.pageCallback(page)
		}
	})

	if !c.options.SinglePage {
		c.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			link := e.Attr("href")

			if strings.HasPrefix(link, "#") ||
				strings.HasPrefix(link, "javascript:") ||
				strings.HasPrefix(link, "mailto:") ||
				strings.HasPrefix(link, "tel:") ||
				strings.HasPrefix(link, "sms:") ||
				strings.HasPrefix(link, "fax:") ||
				strings.HasPrefix(link, "data:") ||
				strings.HasPrefix(link, "file:") {
				return
			}

			if looksLikeEmail(link) || looksLikePhone(link) {
				return
			}

			absoluteURL := e.Request.AbsoluteURL(link)
			if c.isExcludedPath(absoluteURL) {
				return
			}

			//nolint:errcheck
			_ = e.Request.Visit(link)
		})
	}

	c.collector.OnError(func(r *colly.Response, err error) {
		if _, ferr := fmt.Fprintf(c.output, "Error crawling %s: %v\n", r.Request.URL, err); ferr != nil {
			_ = ferr
		}
	})

	c.collector.OnRequest(func(r *colly.Request) {
		if _, ferr := fmt.Fprintf(c.output, "Visiting: %s\n", r.URL.String()); ferr != nil {
			_ = ferr
		}
	})
}

func extractMainContent(e *colly.HTMLElement) string {
	selectors := []string{
		"main",
		"article",
		"[role='main']",
		".content",
		"#content",
		".main-content",
		"#main-content",
		"body",
	}

	for _, selector := range selectors {
		if html, err := e.DOM.Find(selector).First().Html(); err == nil && html != "" {
			return html
		}
	}

	return ""
}

// GetPages returns all crawled pages collected so far.
func (c *Crawler) GetPages() []Page {
	c.pagesMutex.Lock()
	defer c.pagesMutex.Unlock()

	return c.pages
}

func normalizeURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

func (c *Crawler) isExcludedPath(rawURL string) bool {
	if len(c.options.ExcludedPaths) == 0 {
		return false
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	fullPath := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	for _, excluded := range c.options.ExcludedPaths {
		if strings.HasPrefix(fullPath, excluded) || strings.HasPrefix(rawURL, excluded) {
			return true
		}
	}

	return false
}

func looksLikeEmail(s string) bool {
	if !strings.Contains(s, "@") {
		return false
	}

	parts := strings.Split(s, "@")

	return len(parts) == 2 && len(parts[0]) > 0 && len(parts[1]) > 0 && strings.Contains(parts[1], ".")
}

func looksLikePhone(s string) bool {
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, s)

	digitCount := len(cleaned)
	hasPhoneChars := strings.ContainsAny(s, "+()-")

	return digitCount >= 7 && digitCount <= 15 && hasPhoneChars
}
