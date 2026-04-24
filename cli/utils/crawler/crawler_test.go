package crawler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/cli/utils/crawler"
)

func TestNewCrawler_InvalidURL(t *testing.T) {
	_, err := crawler.NewCrawler("://bad", crawler.Options{})
	if err == nil {
		t.Fatal("NewCrawler with invalid URL expected error, got nil")
	}
}

func TestNewCrawler_ValidURL(t *testing.T) {
	c, err := crawler.NewCrawler("https://example.com", crawler.Options{Silent: true})
	if err != nil {
		t.Fatalf("NewCrawler() error = %v", err)
	}

	if c == nil {
		t.Fatal("NewCrawler() returned nil")
	}
}

func TestCrawler_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Test Page</title></head><body><main><p>Hello World</p></main></body></html>`)
	}))
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{
		SinglePage: true,
		Silent:     true,
	})
	if err != nil {
		t.Fatalf("NewCrawler() error = %v", err)
	}

	var pages []crawler.Page
	c.OnPage(func(page crawler.Page) {
		pages = append(pages, page)
	})

	if err := c.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	if pages[0].Title != "Test Page" {
		t.Errorf("Title = %q, want 'Test Page'", pages[0].Title)
	}

	if pages[0].Content == "" {
		t.Error("Content should not be empty")
	}
}

func TestCrawler_ExcludedPath(t *testing.T) {
	visitCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visitCount++
		w.Header().Set("Content-Type", "text/html")

		if r.URL.Path == "/" {
			fmt.Fprintf(w, `<html><head><title>Home</title></head><body>
				<a href="/allowed">Allowed</a>
				<a href="/excluded/page">Excluded</a>
			</body></html>`)
		} else {
			fmt.Fprint(w, `<html><head><title>Sub</title></head><body><p>content</p></body></html>`)
		}
	}))
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{
		MaxDepth:      2,
		Silent:        true,
		ExcludedPaths: []string{srv.URL + "/excluded"},
	})
	if err != nil {
		t.Fatalf("NewCrawler() error = %v", err)
	}

	var pages []crawler.Page
	c.OnPage(func(page crawler.Page) {
		pages = append(pages, page)
	})

	if err := c.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for _, p := range pages {
		if strings.Contains(p.URL, "/excluded") {
			t.Errorf("excluded path was visited: %s", p.URL)
		}
	}
}

func TestCrawler_GetPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>T</title></head><body><p>content</p></body></html>`)
	}))
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{SinglePage: true, Silent: true})
	if err != nil {
		t.Fatalf("NewCrawler() error = %v", err)
	}

	if err := c.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	pages := c.GetPages()
	if len(pages) == 0 {
		t.Error("GetPages() should return at least one page")
	}
}
