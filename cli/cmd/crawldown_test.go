package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// resetCrawldownFlags resets the persistent flags of crawldownCmd to prevent
// cross-test contamination — cobra/pflag does not reset Changed state between Execute calls.
func resetCrawldownFlags() {
	crawldownCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
}

func TestCrawldownCommand_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>My Page</title></head><body><main><h1>Hello</h1><p>World content</p></main></body></html>`)
	}))
	defer srv.Close()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "My Page") {
		t.Errorf("output missing title, got: %s", output)
	}

	if !strings.Contains(output, "Hello") {
		t.Errorf("output missing heading, got: %s", output)
	}
}

func TestCrawldownCommand_CrawlMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Site Home</title></head><body><main><p>Welcome</p></main></body></html>`)
	}))
	defer srv.Close()

	dir := t.TempDir()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --output failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one .md file saved, got none")
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("unexpected file %q (expected .md)", e.Name())
		}

		content, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // test reads known temp files
		if err != nil {
			t.Fatalf("ReadFile(%s) error: %v", e.Name(), err)
		}

		if !strings.Contains(string(content), "Site Home") {
			t.Errorf("file %q missing title 'Site Home'", e.Name())
		}
	}
}

func TestCrawldownCommand_DownloadDocs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/doc/sample.pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4\n%EOF"))
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Docs</title></head><body><main><p>See <a href="/doc/sample.pdf">PDF</a></p></main></body></html>`)
	}))
	defer srv.Close()

	dir := t.TempDir()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, "--download-docs", srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --download-docs failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	foundPDF := false
	foundMD := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pdf") {
			foundPDF = true
		}
		if strings.HasSuffix(e.Name(), ".md") {
			foundMD = true
			content, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // test reads known temp files
			if err != nil {
				t.Fatalf("ReadFile(%s) error: %v", e.Name(), err)
			}
			if !strings.Contains(string(content), "sample.pdf") {
				t.Errorf("markdown file %q did not contain local PDF link", e.Name())
			}
		}
	}

	if !foundPDF {
		t.Fatal("expected downloaded PDF file, got none")
	}
	if !foundMD {
		t.Fatal("expected markdown file, got none")
	}
}

func TestCrawldownCommand_DownloadDocsFromRootWithDepth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/document.pdf/view" {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4\n%EOF"))
			return
		}

		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><title>Page</title></head><body><main><p><a href="/document.pdf/view">Download PDF</a></p></main></body></html>`)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Root</title></head><body><main><p><a href="/page">Page</a></p></main></body></html>`)
	}))
	defer srv.Close()

	dir := t.TempDir()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, "--download-docs", "--depth", "2", srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --download-docs --depth 2 failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	foundPDF := false
	foundPage := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".pdf" {
			foundPDF = true
		}
		if strings.HasSuffix(e.Name(), ".md") {
			foundPage = true
		}
	}

	if !foundPDF {
		t.Fatal("expected downloaded PDF file from root crawl, got none")
	}
	if !foundPage {
		t.Fatal("expected markdown pages from root crawl, got none")
	}
}

func TestCrawldownCommand_OutputFile(t *testing.T) {
	resetCrawldownFlags()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>File Page</title></head><body><main><h1>File Output</h1><p>Written to file</p></main></body></html>`)
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "result.md")

	rootCmd.SetArgs([]string{"crawldown", "--output-file", outputPath, srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --output-file failed: %v", err)
	}

	content, err := os.ReadFile(outputPath) //nolint:gosec // test reads known temp file
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", outputPath, err)
	}

	if !strings.Contains(string(content), "File Page") {
		t.Errorf("output missing title, got: %s", string(content))
	}

	if !strings.Contains(string(content), "File Output") {
		t.Errorf("output missing heading, got: %s", string(content))
	}
}

func TestCrawldownCommand_AllowedPath(t *testing.T) {
	resetCrawldownFlags()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" || r.URL.Path == "" {
			fmt.Fprint(w, `<html><head><title>Home</title></head><body><main>
				<a href="/good-practices/page1">Good</a>
				<a href="/other/page">Other</a>
			</main></body></html>`)
		} else {
			fmt.Fprint(w, `<html><head><title>Sub</title></head><body><main><p>content</p></main></body></html>`)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, "--allowed-path", srv.URL + "/good-practices", srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --allowed-path failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	// Should have the root page (always visited) and /good-practices/page1,
	// but NOT /other/page.
	for _, e := range entries {
		if strings.Contains(e.Name(), "other") {
			t.Errorf("non-allowed path was crawled and saved as %q", e.Name())
		}
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 files (root + allowed page), got %d: %v", len(entries), entries)
	}
}

func TestCrawldownCommand_OutputAndOutputFileConflict(t *testing.T) {
	resetCrawldownFlags()

	exited := -1
	origExit := exit
	exit = func(code int) {
		exited = code
		panic("exit")
	}
	defer func() {
		exit = origExit
	}()
	defer func() {
		if r := recover(); r != nil && r != "exit" {
			panic(r)
		}
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", "/tmp/out", "--output-file", "/tmp/out.md", "http://example.com"})
	_ = rootCmd.Execute()

	if exited != 1 {
		t.Fatalf("expected exit code 1, got %v", exited)
	}
}

func TestCrawldownCommand_NoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"crawldown"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing URL argument, got nil")
	}
}
