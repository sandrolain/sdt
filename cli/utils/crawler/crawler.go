package crawler

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/gocolly/colly"
	"github.com/sandrolain/sdt/cli/utils/converter"
)

// maxRetries bounds the transient-failure retries per crawl request.
const maxRetries = 3

// maxDocumentBytes caps a downloaded document body (colly's implicit default is
// 10 MB); a body over the cap fails the request, which OnError reports.
const maxDocumentBytes = 100 << 20

// Page represents a crawled web page.
type Page struct {
	URL         string
	Title       string
	Content     string
	Description string
	Keywords    string
	Date        string
	Author      string
}

// Options defines crawler configuration.
type Options struct {
	MaxDepth            int
	AllowedDomains      []string
	UserAgent           string
	IgnoreRobotsTxt     bool
	FollowExternalLinks bool
	SinglePage          bool
	DownloadDocuments   bool
	// Sitemap seeds the crawl from the site's robots.txt sitemaps (or
	// /sitemap.xml), bounded and best-effort; opt-in, crawl mode only.
	Sitemap            bool
	RequestTimeout     int
	RequestDelay       int
	ExcludedPaths      []string
	AllowedPaths       []string
	AllowedPathRegexes []string
	// Silent suppresses the periodic crawl progress goroutine; individual
	// visiting/error lines are gated by the shared logger's level instead.
	Silent bool
}

// PageCallback is called when a page is successfully crawled.
type PageCallback func(page Page)

// Document represents a crawled non-HTML document.
type Document struct {
	URL         string
	ContentType string
	// Disposition is the raw Content-Disposition header, used to resolve a
	// download's filename (RFC 5987).
	Disposition string
	Body        []byte
}

// DocumentCallback is called when a downloadable document is successfully crawled.
type DocumentCallback func(doc Document)

// Crawler handles web crawling operations.
type Crawler struct {
	collector         *colly.Collector
	documentCollector *colly.Collector
	pages             []Page
	pagesMutex        sync.Mutex
	baseURL           *url.URL
	options           Options
	pageCallback      PageCallback
	documentCallback  DocumentCallback
	requestsTotal     int64
	responsesTotal    int64
	errorsTotal       int64
	inFlight          int64
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

	docCollector := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.UserAgent(opts.UserAgent),
		colly.Async(true),
	)

	docCollector.SetRequestTimeout(time.Duration(opts.RequestTimeout) * time.Second)
	if opts.RequestDelay > 0 {
		if err := docCollector.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Delay:       time.Duration(opts.RequestDelay) * time.Second,
			RandomDelay: time.Duration(opts.RequestDelay/2) * time.Second,
			Parallelism: 2,
		}); err != nil {
			return nil, fmt.Errorf("set document rate limit: %w", err)
		}
	} else {
		if err := docCollector.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: 2,
		}); err != nil {
			return nil, fmt.Errorf("set document limit: %w", err)
		}
	}

	if opts.IgnoreRobotsTxt {
		c.IgnoreRobotsTxt = true
		docCollector.IgnoreRobotsTxt = true
	}

	// Decode non-UTF8 pages (colly's default is off); cap a document body so a
	// large PDF fails loudly instead of silently truncating at colly's default.
	c.DetectCharset = true
	docCollector.DetectCharset = true
	docCollector.MaxBodySize = maxDocumentBytes

	cr := &Crawler{
		collector:         c,
		documentCollector: docCollector,
		pages:             []Page{},
		baseURL:           parsedURL,
		options:           opts,
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

	// Seed the crawl from the site's sitemaps (bounded, best-effort) before the
	// base URL, so a docs site is covered beyond the base page's link graph.
	if c.options.Sitemap && !c.options.SinglePage {
		for _, u := range discoverSitemapURLs(c.baseURL, c.options.UserAgent, time.Duration(c.options.RequestTimeout)*time.Second) {
			slog.Debug("sitemap seed", "url", u)
			//nolint:errcheck
			_ = c.collector.Visit(u)
		}
	}

	done := make(chan struct{})
	if !c.options.Silent {
		go c.reportProgress(done)
	}
	defer close(done)

	if err := c.collector.Visit(c.baseURL.String()); err != nil {
		return fmt.Errorf("start crawling: %w", err)
	}

	c.collector.Wait()
	if c.documentCollector != nil {
		c.documentCollector.Wait()
	}

	return nil
}

// Errors returns the number of request/response errors seen so far.
func (c *Crawler) Errors() int64 {
	return atomic.LoadInt64(&c.errorsTotal)
}

//nolint:gocognit,gocyclo // pre-existing high-complexity callback registration; kept as-is to avoid behavioral changes
func (c *Crawler) setupCallbacks() {
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		normalizedURL := normalizeURL(e.Request.URL.String())

		description := e.ChildAttr("meta[name=description]", "content")
		if description == "" {
			description = e.ChildAttr("meta[property=og:description]", "content")
		}

		keywords := e.ChildAttr("meta[name=keywords]", "content")

		date := e.ChildAttr("meta[name=date]", "content")
		if date == "" {
			date = e.ChildAttr("meta[property=article:published_time]", "content")
		}
		if date == "" {
			date = e.ChildAttr("meta[property=article:modified_time]", "content")
		}

		author := e.ChildAttr("meta[name=author]", "content")

		page := Page{
			URL:         normalizedURL,
			Title:       e.ChildText("title"),
			Content:     extractMainContent(e),
			Description: description,
			Keywords:    keywords,
			Date:        date,
			Author:      author,
		}

		c.pagesMutex.Lock()
		c.pages = append(c.pages, page)
		c.pagesMutex.Unlock()

		if c.pageCallback != nil {
			c.pageCallback(page)
		}
	})

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
			slog.Debug("link denied", "url", absoluteURL, "reason", "EXCLUDE_PATTERN")
			return
		}
		if !c.isAllowedPath(absoluteURL) {
			slog.Debug("link denied", "url", absoluteURL, "reason", "INCLUDE_PATTERN")
			return
		}

		if isDocumentLink(link) {
			if !c.options.DownloadDocuments {
				slog.Debug("link denied", "url", absoluteURL, "reason", "DOCUMENT_NOT_DOWNLOADING")
				return
			}

			if c.documentCollector != nil {
				if err := c.documentCollector.Visit(absoluteURL); err != nil {
					atomic.AddInt64(&c.errorsTotal, 1)
				}
			}
			return
		}

		// Single-page mode captures exactly one page; only follow page links
		// when actually crawling. Document links above are still honoured.
		if c.options.SinglePage {
			slog.Debug("link denied", "url", absoluteURL, "reason", "SINGLE_PAGE")
			return
		}

		//nolint:errcheck
		_ = e.Request.Visit(link)
	})

	c.collector.OnResponse(func(r *colly.Response) {
		atomic.AddInt64(&c.responsesTotal, 1)
		atomic.AddInt64(&c.inFlight, -1)

		if c.options.DownloadDocuments && isDocumentResponse(r) {
			doc := Document{
				URL:         normalizeURL(r.Request.URL.String()),
				ContentType: r.Headers.Get("Content-Type"),
				Disposition: r.Headers.Get("Content-Disposition"),
				Body:        r.Body,
			}

			if c.documentCallback != nil {
				c.documentCallback(doc)
			}
		}
	})

	if c.documentCollector != nil {
		c.documentCollector.OnResponse(func(r *colly.Response) {
			atomic.AddInt64(&c.responsesTotal, 1)
			atomic.AddInt64(&c.inFlight, -1)

			if c.options.DownloadDocuments && isDocumentResponse(r) {
				doc := Document{
					URL:         normalizeURL(r.Request.URL.String()),
					ContentType: r.Headers.Get("Content-Type"),
					Body:        r.Body,
				}

				if c.documentCallback != nil {
					c.documentCallback(doc)
				}
			}
		})

		c.documentCollector.OnError(func(r *colly.Response, err error) {
			atomic.AddInt64(&c.inFlight, -1)

			if retryTransient(r, err) {
				slog.Debug("retrying crawl", "url", r.Request.URL.String(), "status", r.StatusCode)
				return
			}

			atomic.AddInt64(&c.errorsTotal, 1)
			slog.Warn("error crawling", "url", r.Request.URL.String(), "err", err)
		})

		c.documentCollector.OnRequest(func(r *colly.Request) {
			atomic.AddInt64(&c.requestsTotal, 1)
			atomic.AddInt64(&c.inFlight, 1)

			slog.Info("visiting", "url", r.URL.String())
		})
	}

	c.collector.OnError(func(r *colly.Response, err error) {
		atomic.AddInt64(&c.inFlight, -1)

		if retryTransient(r, err) {
			slog.Debug("retrying crawl", "url", r.Request.URL.String(), "status", r.StatusCode)
			return
		}

		atomic.AddInt64(&c.errorsTotal, 1)
		slog.Warn("error crawling", "url", r.Request.URL.String(), "err", err)
	})

	c.collector.OnRequest(func(r *colly.Request) {
		atomic.AddInt64(&c.requestsTotal, 1)
		atomic.AddInt64(&c.inFlight, 1)

		slog.Info("visiting", "url", r.URL.String())
	})
}

func (c *Crawler) reportProgress(done <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			inFlight := atomic.LoadInt64(&c.inFlight)
			if inFlight <= 0 {
				continue
			}

			requests := atomic.LoadInt64(&c.requestsTotal)
			responses := atomic.LoadInt64(&c.responsesTotal)
			errors := atomic.LoadInt64(&c.errorsTotal)

			slog.Info("crawl progress",
				"requested", requests, "completed", responses,
				"errors", errors, "in-flight", inFlight)
		}
	}
}

func extractMainContent(e *colly.HTMLElement) string {
	// Try readability algorithm first (reader mode) — strips layout, nav, sidebars, etc.
	if content, err := extractReadableContent(e); err == nil && content != "" {
		return content
	}

	// Fallback to selector-based extraction
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

func extractReadableContent(e *colly.HTMLElement) (string, error) {
	if e.Response == nil || len(e.Response.Body) == 0 || e.Request == nil || e.Request.URL == nil {
		return "", fmt.Errorf("missing response or URL")
	}

	article, err := readability.FromReader(
		strings.NewReader(string(e.Response.Body)),
		e.Request.URL,
	)
	if err != nil {
		return "", fmt.Errorf("readability failed: %w", err)
	}

	var buf strings.Builder
	if err := article.RenderHTML(&buf); err != nil {
		return "", fmt.Errorf("render readability HTML: %w", err)
	}

	content := strings.TrimSpace(buf.String())
	if len(content) < 50 {
		return "", fmt.Errorf("readability content too short (%d chars)", len(content))
	}

	return content, nil
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

	// Strip the fragment: `page#a` and `page#b` are the same capture, and a
	// fragment never reaches the server. A host is case-insensitive.
	parsedURL.Fragment = ""
	parsedURL.RawFragment = ""
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	query := parsedURL.Query()
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

func matchURLPrefix(rawURL, prefix string) bool {
	if strings.HasPrefix(prefix, "http://") || strings.HasPrefix(prefix, "https://") {
		return strings.HasPrefix(rawURL, prefix)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return strings.HasPrefix(parsedURL.Path, prefix)
}

func (c *Crawler) isExcludedPath(rawURL string) bool {
	if len(c.options.ExcludedPaths) == 0 {
		return false
	}

	for _, excluded := range c.options.ExcludedPaths {
		if matchURLPrefix(rawURL, excluded) {
			return true
		}
	}

	return false
}

func matchURLRegex(rawURL, pattern string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return re.MatchString(parsedURL.Path)
}

func (c *Crawler) isAllowedPath(rawURL string) bool {
	hasPrefixes := len(c.options.AllowedPaths) > 0
	hasRegexes := len(c.options.AllowedPathRegexes) > 0

	if !hasPrefixes && !hasRegexes {
		return true
	}

	if hasPrefixes {
		for _, allowed := range c.options.AllowedPaths {
			if matchURLPrefix(rawURL, allowed) {
				return true
			}
		}
	}

	if hasRegexes {
		for _, allowed := range c.options.AllowedPathRegexes {
			if matchURLRegex(rawURL, allowed) {
				return true
			}
		}
	}

	return false
}

// isTransient reports whether a failed request is worth retrying: a transport
// error, HTTP 429 or 5xx. A 4xx that is not 429 is not.
func isTransient(status int, err error) bool {
	return err != nil || status == http.StatusTooManyRequests || status >= 500
}

// retryTransient re-queues a request that failed transiently — a transport
// error, HTTP 429 or 5xx — up to maxRetries, with a short linear backoff. A 4xx
// that is not 429 is not retried. Returns true when the request was re-queued.
func retryTransient(r *colly.Response, err error) bool {
	if !isTransient(r.StatusCode, err) {
		return false
	}
	attempts := 0
	if v := r.Request.Ctx.Get("retry"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			attempts = n
		}
	}
	if attempts >= maxRetries {
		return false
	}
	r.Request.Ctx.Put("retry", strconv.Itoa(attempts+1))
	time.Sleep(time.Duration(attempts+1) * 250 * time.Millisecond)
	//nolint:errcheck
	_ = r.Request.Retry()
	return true
}

// OnDocument registers a callback invoked for each downloaded non-HTML document.
func (c *Crawler) OnDocument(callback DocumentCallback) {
	c.documentCallback = callback
}

func isDocumentLink(link string) bool {
	parsed, err := url.Parse(link)
	if err != nil {
		return isDocumentPath(strings.ToLower(link))
	}

	return isDocumentPath(strings.ToLower(parsed.Path))
}

func isDocumentPath(path string) bool {
	return converter.DocumentExtension(path) != ""
}

func isDocumentResponse(r *colly.Response) bool {
	// An explicit download wins even over an HTML content type.
	if strings.Contains(strings.ToLower(r.Headers.Get("Content-Disposition")), "attachment") {
		return true
	}

	if isHTMLResponse(r) {
		return false
	}

	contentType := strings.ToLower(r.Headers.Get("Content-Type"))
	if strings.Contains(contentType, "application/pdf") ||
		strings.Contains(contentType, "application/msword") ||
		strings.Contains(contentType, "application/vnd.openxmlformats-officedocument") ||
		strings.Contains(contentType, "application/rtf") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/markdown") ||
		strings.Contains(contentType, "text/csv") ||
		strings.Contains(contentType, "application/vnd.oasis.opendocument") {
		return true
	}

	if strings.Contains(contentType, "application/octet-stream") {
		return isDocumentLink(r.Request.URL.String())
	}

	return isDocumentLink(r.Request.URL.String())
}

func isHTMLResponse(r *colly.Response) bool {
	contentType := strings.ToLower(r.Headers.Get("Content-Type"))
	return strings.HasPrefix(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml")
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
