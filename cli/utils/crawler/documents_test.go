package crawler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/cli/utils/crawler"
)

// pageWithDocument serves one HTML page linking to /doc, forwarding the
// document request to docHandler.
func pageWithDocument(t *testing.T, docHandler http.HandlerFunc) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/doc", docHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Docs</title></head><body><main>` +
			`<p>` + strings.Repeat("content ", 10) + `</p>` +
			`<a href="/doc">the document</a></main></body></html>`))
	})

	return httptest.NewServer(mux)
}

func TestCrawler_DocumentKeepsItsDisposition(t *testing.T) {
	srv := pageWithDocument(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf")
		_, _ = w.Write([]byte("%PDF-1.4"))
	})
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{
		MaxDepth:          2,
		DownloadDocuments: true,
		Silent:            true,
	})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}

	var docs []crawler.Document
	c.OnDocument(func(doc crawler.Document) {
		docs = append(docs, doc)
	})

	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Disposition, "filename*=UTF-8") {
		t.Errorf("document lost its Content-Disposition: %q", docs[0].Disposition)
	}
}

func TestCrawler_RefusesAnOversizeDocument(t *testing.T) {
	srv := pageWithDocument(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte(strings.Repeat("p", 100)))
	})
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{
		MaxDepth:          2,
		DownloadDocuments: true,
		MaxDocumentBytes:  10,
		Silent:            true,
	})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}

	var docs []crawler.Document
	c.OnDocument(func(doc crawler.Document) {
		docs = append(docs, doc)
	})

	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(docs) != 0 {
		t.Fatalf("an oversize document reached the callback: %d", len(docs))
	}
	if c.Errors() == 0 {
		t.Error("an oversize document should count as a failure")
	}
	if got := c.RefusedDocuments(); got != 1 {
		t.Errorf("RefusedDocuments() = %d, want 1", got)
	}
}

// TestCrawler_AttachmentWithoutExtensionIsADocument covers the page collector's
// document branch: a URL with no document extension is fetched as a page, then
// recognised by its Content-Disposition.
func TestCrawler_AttachmentWithoutExtensionIsADocument(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/download", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
		_, _ = w.Write([]byte("%PDF-1.4"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>` + strings.Repeat("content ", 10) + `</p>` +
			`<a href="/download?file=1">download</a></main></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{
		MaxDepth:          2,
		DownloadDocuments: true,
		Silent:            true,
	})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}

	var docs []crawler.Document
	c.OnDocument(func(doc crawler.Document) {
		docs = append(docs, doc)
	})

	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Disposition, "report.pdf") {
		t.Errorf("attachment lost its disposition: %q", docs[0].Disposition)
	}
}

func TestCrawler_SinglePageFollowsNoPageLinks(t *testing.T) {
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>` + strings.Repeat("content ", 10) + `</p>` +
			`<a href="/other">other</a></main></body></html>`))
	})
	mux.HandleFunc("/other", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main><p>other page</p></main></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := crawler.NewCrawler(srv.URL, crawler.Options{SinglePage: true, Silent: true})
	if err != nil {
		t.Fatalf("NewCrawler: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if hits != 0 {
		t.Errorf("single-page mode followed %d page links", hits)
	}
}
