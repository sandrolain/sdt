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
)

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

func TestCrawldownCommand_NoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"crawldown"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing URL argument, got nil")
	}
}
