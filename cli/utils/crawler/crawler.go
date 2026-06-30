package crawler

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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
	DownloadDocuments   bool
	RequestTimeout      int
	RequestDelay        int
	ExcludedPaths       []string
	AllowedPaths        []string
	Silent              bool
}

// PageCallback is called when a page is successfully crawled.
type PageCallback func(page Page)

// Document represents a crawled non-HTML document.
type Document struct {
	URL         string
	ContentType string
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
	output            io.Writer
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
		_ = docCollector.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Delay:       time.Duration(opts.RequestDelay) * time.Second,
			RandomDelay: time.Duration(opts.RequestDelay/2) * time.Second,
			Parallelism: 2,
		})
	} else {
		_ = docCollector.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: 2,
		})
	}

	if opts.IgnoreRobotsTxt {
		c.IgnoreRobotsTxt = true
		docCollector.IgnoreRobotsTxt = true
	}

	output := io.Writer(os.Stdout)
	if opts.Silent {
		output = io.Discard
	}

	cr := &Crawler{
		collector:         c,
		documentCollector: docCollector,
		pages:             []Page{},
		baseURL:           parsedURL,
		options:           opts,
		output:            output,
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
			if c.isExcludedPath(absoluteURL) || !c.isAllowedPath(absoluteURL) {
				return
			}

			if isDocumentLink(link) {
				if !c.options.DownloadDocuments {
					return
				}

				if c.documentCollector != nil {
					_ = c.documentCollector.Visit(absoluteURL)
				}
				return
			}

			//nolint:errcheck
			_ = e.Request.Visit(link)
		})
	}

	c.collector.OnResponse(func(r *colly.Response) {
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
			atomic.AddInt64(&c.errorsTotal, 1)
			atomic.AddInt64(&c.inFlight, -1)

			if _, ferr := fmt.Fprintf(c.output, "Error crawling %s: %v\n", r.Request.URL, err); ferr != nil {
				_ = ferr
			}
		})

		c.documentCollector.OnRequest(func(r *colly.Request) {
			atomic.AddInt64(&c.requestsTotal, 1)
			atomic.AddInt64(&c.inFlight, 1)

			if _, ferr := fmt.Fprintf(c.output, "Visiting: %s\n", r.URL.String()); ferr != nil {
				_ = ferr
			}
		})
	}

	c.collector.OnError(func(r *colly.Response, err error) {
		atomic.AddInt64(&c.errorsTotal, 1)
		atomic.AddInt64(&c.inFlight, -1)

		if _, ferr := fmt.Fprintf(c.output, "Error crawling %s: %v\n", r.Request.URL, err); ferr != nil {
			_ = ferr
		}
	})

	c.collector.OnRequest(func(r *colly.Request) {
		atomic.AddInt64(&c.requestsTotal, 1)
		atomic.AddInt64(&c.inFlight, 1)

		if _, ferr := fmt.Fprintf(c.output, "Visiting: %s\n", r.URL.String()); ferr != nil {
			_ = ferr
		}
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

			if _, err := fmt.Fprintf(c.output, "Progress: requested=%d completed=%d errors=%d in-flight=%d\n", requests, responses, errors, inFlight); err != nil {
				_ = err
			}
		}
	}
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

func (c *Crawler) isAllowedPath(rawURL string) bool {
	if len(c.options.AllowedPaths) == 0 {
		return true
	}

	for _, allowed := range c.options.AllowedPaths {
		if matchURLPrefix(rawURL, allowed) {
			return true
		}
	}

	return false
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
	extensions := []string{
		".pdf",
		".doc",
		".docx",
		".xls",
		".xlsx",
		".ppt",
		".pptx",
		".odt",
		".ods",
		".odp",
		".rtf",
		".txt",
		".md",
		".markdown",
		".csv",
	}

	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) || strings.Contains(path, ext+"/") || strings.Contains(path, ext+"?") || strings.Contains(path, ext+"#") {
			return true
		}
	}

	return false
}

func isDocumentResponse(r *colly.Response) bool {
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
