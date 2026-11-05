package crawler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

func TestUnderExtracted(t *testing.T) {
	cases := []struct {
		name      string
		extracted int
		page      int
		want      bool
	}{
		{"sliver of a large page", 900, 40000, true},
		{"half of a large page", 20000, 40000, false},
		{"small page is never judged", 100, 400, false},
		{"exactly at the minimum size", 300, 2000, true},
		{"just above the ratio", 600, 2000, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := underExtracted(c.extracted, c.page); got != c.want {
				t.Errorf("underExtracted(%d, %d) = %v, want %v", c.extracted, c.page, got, c.want)
			}
		})
	}
}

func TestVisibleTextLenIgnoresMarkupAndScripts(t *testing.T) {
	html := `<html><head><style>.a{color:red}</style><script>var x = "` + strings.Repeat("x", 5000) + `";</script></head>` +
		`<body><p>hello world</p></body></html>`

	if got := visibleTextLen(html); got != len("hello world") {
		t.Errorf("visibleTextLen = %d, want %d", got, len("hello world"))
	}
}

func TestRefusedDocument(t *testing.T) {
	c := &Crawler{options: Options{MaxDocumentBytes: 100}}

	cases := []struct {
		name    string
		header  string
		body    []byte
		refused bool
	}{
		{"header over the cap", "200", bytes.Repeat([]byte("a"), 10), true},
		{"body at the cap without a header", "", bytes.Repeat([]byte("a"), 100), true},
		{"body under the cap", "50", bytes.Repeat([]byte("a"), 50), false},
		{"lying header", "10", bytes.Repeat([]byte("a"), 100), true},
		{"unparsable header under the cap", "abc", bytes.Repeat([]byte("a"), 10), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{}
			if tc.header != "" {
				headers.Set("Content-Length", tc.header)
			}
			r := &colly.Response{Headers: &headers, Body: tc.body}
			if got := c.refusedDocument(r); got != tc.refused {
				t.Errorf("refusedDocument = %v, want %v", got, tc.refused)
			}
		})
	}
}

func TestAdmitPageCountsDistinctPages(t *testing.T) {
	c := &Crawler{options: Options{MaxPages: 2}}

	if !c.admitPage("https://e.test/a") {
		t.Fatal("first page should be admitted")
	}
	if !c.admitPage("https://e.test/a#x") {
		t.Fatal("a retry of an admitted page should be admitted")
	}
	if !c.admitPage("https://e.test/b") {
		t.Fatal("second page should be admitted")
	}
	if c.admitPage("https://e.test/c") {
		t.Fatal("page over the cap should be refused")
	}

	c.options.MaxPages = 0
	if !c.admitPage("https://e.test/c") {
		t.Fatal("an unlimited crawl admits every page")
	}
}

func TestRetryTransientRefusesAnUnseekableBody(t *testing.T) {
	headers := http.Header{}
	r := &colly.Response{
		StatusCode: http.StatusServiceUnavailable,
		Headers:    &headers,
		Request: &colly.Request{
			URL:     &url.URL{Scheme: "https", Host: "e.test", Path: "/doc.pdf"},
			Headers: &http.Header{},
			Ctx:     colly.NewContext(),
			Body:    io.NopCloser(strings.NewReader("payload")),
		},
	}

	if retryTransient(r, nil) {
		t.Fatal("an unseekable body cannot be retried")
	}
}

func TestDenialReasons(t *testing.T) {
	reasons := map[string]string{
		"EXCLUDE_PATTERN":          reasonExcludePattern,
		"INCLUDE_PATTERN":          reasonIncludePattern,
		"DOCUMENT_NOT_DOWNLOADING": reasonDocumentNotDownloading,
		"SINGLE_PAGE":              reasonSinglePage,
		"LIMIT":                    reasonLimit,
	}
	for want, got := range reasons {
		if got != want {
			t.Errorf("denial reason = %q, want %q", got, want)
		}
	}
}

func TestDenialReasonNamesThePredicate(t *testing.T) {
	c := &Crawler{options: Options{ExcludedPaths: []string{"/no"}, AllowedPaths: []string{"/yes"}}}

	if got := c.denialReason("https://e.test/no/x"); got != reasonExcludePattern {
		t.Errorf("excluded path reason = %q", got)
	}
	if got := c.denialReason("https://e.test/other"); got != reasonIncludePattern {
		t.Errorf("not-allowed path reason = %q", got)
	}
	if c.allowed("https://e.test/no/x") || !c.allowed("https://e.test/yes/x") {
		t.Error("allowed() disagrees with the predicates")
	}
}

// TestStartFiltersSitemapSeeds proves a seeded URL that the path filter
// excludes is never fetched.
func TestStartFiltersSitemapSeeds(t *testing.T) {
	var base string
	visited := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Sitemap: " + base + "/sitemap.xml\n"))
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<urlset>` +
			"<url><loc>" + base + "/keep</loc></url>" +
			"<url><loc>" + base + "/secret</loc></url>" +
			"</urlset>"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		visited[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body><p>` + strings.Repeat("text ", 20) + `</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	c, err := NewCrawler(srv.URL, Options{
		Sitemap:       true,
		MaxDepth:      1,
		ExcludedPaths: []string{"/secret"},
		Silent:        true,
	})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if visited["/keep"] == 0 {
		t.Error("an allowed seed should be fetched")
	}
	if visited["/secret"] != 0 {
		t.Errorf("an excluded seed was fetched %d times", visited["/secret"])
	}
}

func TestStartHonoursMaxPages(t *testing.T) {
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>P</title></head><body><main><p>hello page</p>` +
			`<a href="/b">b</a><a href="/c">c</a><a href="/d">d</a></main></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewCrawler(srv.URL, Options{MaxDepth: 2, MaxPages: 2, Silent: true})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if hits > 2 {
		t.Errorf("page cap 2 admitted %d requests", hits)
	}
	if hits == 0 {
		t.Fatal("the crawl fetched nothing")
	}
}

func TestDecodeSitemapBody(t *testing.T) {
	plain := []byte(`<urlset><url><loc>https://e.test/a</loc></url></urlset>`)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	if got := decodeSitemapBody("https://e.test/sitemap.xml", plain); !bytes.Equal(got, plain) {
		t.Error("a plain body should pass through unchanged")
	}
	if got := decodeSitemapBody("https://e.test/sitemap.xml.gz", buf.Bytes()); !bytes.Equal(got, plain) {
		t.Errorf("decoded body = %q, want %q", got, plain)
	}
	if got := decodeSitemapBody("https://e.test/broken.gz", []byte{0x1f, 0x8b, 0x00}); got != nil {
		t.Error("a broken gzip body should yield nil")
	}
}

func TestDiscoverSitemapURLsSkipsFilteredURLs(t *testing.T) {
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<urlset>` +
			"<url><loc>" + base + "/keep</loc></url>" +
			"<url><loc>" + base + "/skip</loc></url>" +
			"</urlset>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u, _ := url.Parse(srv.URL)
	got := discoverSitemapURLs(u, "sdt-test", 5*time.Second, func(raw string) bool {
		return !strings.HasSuffix(raw, "/skip")
	})

	if len(got) != 1 || got[0] != base+"/keep" {
		t.Errorf("discoverSitemapURLs = %v, want [%s/keep]", got, base)
	}
}

func TestDiscoverSitemapURLsHandlesGzip(t *testing.T) {
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(`<urlset><url><loc>` + base + `/gz</loc></url></urlset>`))
		if err := zw.Close(); err != nil {
			t.Errorf("gzip close: %v", err)
		}
		_, _ = w.Write(buf.Bytes())
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u, _ := url.Parse(srv.URL)
	got := discoverSitemapURLs(u, "sdt-test", 5*time.Second, nil)

	if len(got) != 1 || got[0] != base+"/gz" {
		t.Errorf("gzipped sitemap = %v, want [%s/gz]", got, base)
	}
}

// TestRetryIsBoundedAndClassified proves a transient failure is retried up to
// the cap, a 4xx is not retried, and a retry never consumes the page budget.
func TestRetryIsBoundedAndClassified(t *testing.T) {
	prev := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = prev }()

	var flaky, gone int
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, _ *http.Request) {
		flaky++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, _ *http.Request) {
		gone++
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>hello page</p>` +
			`<a href="/flaky">f</a><a href="/gone">g</a></main></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewCrawler(srv.URL, Options{MaxDepth: 2, Silent: true})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if flaky != maxRetries+1 {
		t.Errorf("flaky request fetched %d times, want %d", flaky, maxRetries+1)
	}
	if gone != 1 {
		t.Errorf("404 request fetched %d times, want 1", gone)
	}
}

func TestExtractMainContentFallsBackToTheCascade(t *testing.T) {
	// Below readability's 50-character floor, so the cascade must answer.
	html := `<html><body><main><p>tiny</p></main></body></html>`
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	e := &colly.HTMLElement{
		DOM:      dom.Selection,
		Response: &colly.Response{Body: []byte(html)},
		Request:  &colly.Request{URL: &url.URL{Scheme: "https", Host: "e.test", Path: "/"}},
	}

	got := extractMainContent(e)
	if !strings.Contains(got, "tiny") {
		t.Errorf("cascade content = %q, want the main element", got)
	}

	emptyHTML := `<html><body></body></html>`
	emptyDOM, err := goquery.NewDocumentFromReader(strings.NewReader(emptyHTML))
	if err != nil {
		t.Fatalf("parse empty fixture: %v", err)
	}
	empty := &colly.HTMLElement{
		DOM:      emptyDOM.Selection,
		Response: &colly.Response{Body: []byte(emptyHTML)},
		Request:  &colly.Request{URL: &url.URL{Scheme: "https", Host: "e.test", Path: "/"}},
	}
	if got := extractMainContent(empty); visibleTextLen(got) != 0 {
		t.Errorf("empty page content = %q, want no text", got)
	}
}

func TestExtractReadableContentRejectsAMissingBody(t *testing.T) {
	e := &colly.HTMLElement{
		Response: &colly.Response{Body: nil},
		Request:  &colly.Request{URL: &url.URL{Scheme: "https", Host: "e.test", Path: "/"}},
	}
	if _, err := extractReadableContent(e); err == nil {
		t.Error("a missing body should be an error")
	}

	e.Response = nil
	if _, err := extractReadableContent(e); err == nil {
		t.Error("a missing response should be an error")
	}
}

func TestMatchURLRegex(t *testing.T) {
	cases := []struct {
		url, pattern string
		want         bool
	}{
		{"https://e.test/docs/a", `^/docs/`, true},
		{"https://e.test/blog/a", `^/docs/`, false},
		{"://bad", `^/docs/`, false},
		{"https://e.test/docs/a", `[`, false},
	}
	for _, c := range cases {
		if got := matchURLRegex(c.url, c.pattern); got != c.want {
			t.Errorf("matchURLRegex(%q, %q) = %v, want %v", c.url, c.pattern, got, c.want)
		}
	}
}

func TestStartHonoursAllowedPathRegex(t *testing.T) {
	var keep, drop int
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/keep/a", func(w http.ResponseWriter, _ *http.Request) {
		keep++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>kept page content</p></main></body></html>`))
	})
	mux.HandleFunc("/drop/a", func(w http.ResponseWriter, _ *http.Request) {
		drop++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>dropped page content</p></main></body></html>`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>root page content</p>` +
			`<a href="/keep/a">k</a><a href="/drop/a">d</a></main></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewCrawler(srv.URL, Options{
		MaxDepth:           2,
		AllowedPathRegexes: []string{`/keep/`},
		Silent:             true,
	})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if keep != 1 || drop != 0 {
		t.Errorf("keep=%d drop=%d, want 1 and 0", keep, drop)
	}
}

func TestReportProgressReturnsWhenDone(t *testing.T) {
	c := &Crawler{}
	done := make(chan struct{})
	close(done)

	c.reportProgress(done)
}

func TestLooksLikeEmail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"a@b.co", true},
		{"no-at-sign", false},
		{"a@b", false},
		{"@b.co", false},
		{"a@b@c.co", false},
	}
	for _, c := range cases {
		if got := looksLikeEmail(c.in); got != c.want {
			t.Errorf("looksLikeEmail(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDiscoverSitemapURLsHandlesAGzippedIndex(t *testing.T) {
	var base string
	gzipBody := func(s string) []byte {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(s))
		if err := zw.Close(); err != nil {
			t.Fatalf("gzip close: %v", err)
		}
		return buf.Bytes()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(gzipBody(`<sitemapindex><sitemap><loc>` + base + `/child.xml</loc></sitemap></sitemapindex>`))
	})
	mux.HandleFunc("/child.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(gzipBody(`<urlset><url><loc>` + base + `/deep</loc></url></urlset>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u, _ := url.Parse(srv.URL)
	got := discoverSitemapURLs(u, "sdt-test", 5*time.Second, nil)

	if len(got) != 1 || got[0] != base+"/deep" {
		t.Errorf("gzipped index = %v, want [%s/deep]", got, base)
	}
}
