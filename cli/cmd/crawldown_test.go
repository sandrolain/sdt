package cmd

import (
	"bytes"
	"encoding/json"
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

	t.Run("full URL prefix", func(t *testing.T) {
		resetCrawldownFlags()
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

		for _, e := range entries {
			if strings.Contains(e.Name(), "other") {
				t.Errorf("non-allowed path was crawled and saved as %q", e.Name())
			}
		}
		if len(entries) < 2 {
			t.Errorf("expected at least 2 files (root + allowed page), got %d: %v", len(entries), entries)
		}
	})

	t.Run("relative path prefix", func(t *testing.T) {
		resetCrawldownFlags()
		dir := t.TempDir()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(new(bytes.Buffer))
		defer func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
		}()

		rootCmd.SetArgs([]string{"crawldown", "--output", dir, "--allowed-path", "/good-practices", srv.URL})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("crawldown --allowed-path failed: %v", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir(%s) error: %v", dir, err)
		}

		for _, e := range entries {
			if strings.Contains(e.Name(), "other") {
				t.Errorf("non-allowed path was crawled and saved as %q", e.Name())
			}
		}
		if len(entries) < 2 {
			t.Errorf("expected at least 2 files (root + allowed page), got %d: %v", len(entries), entries)
		}
	})
}

func TestCrawldownCommand_AllowedPathRegex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" || r.URL.Path == "" {
			fmt.Fprint(w, `<html><head><title>Home</title></head><body><main>
				<a href="/blog/page1">Blog</a>
				<a href="/docs/page">Docs</a>
				<a href="/other/page">Other</a>
			</main></body></html>`)
		} else {
			fmt.Fprint(w, `<html><head><title>Sub</title></head><body><main><p>content</p></main></body></html>`)
		}
	}))
	defer srv.Close()

	resetCrawldownFlags()
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, "--allowed-path-regex", `^/blog/`, srv.URL})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("crawldown --allowed-path-regex failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	for _, e := range entries {
		if strings.Contains(e.Name(), "docs") || strings.Contains(e.Name(), "other") {
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

func TestResolveCrawldownMode(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		outputFile string
		urls       int
		want       crawldownMode
		wantErr    bool
	}{
		{"stdout single", "", "", 1, modeStdout, false},
		{"stdout many", "", "", 3, modeStdout, false},
		{"file single", "", "out.md", 1, modeFile, false},
		{"file many errors", "", "out.md", 2, modeStdout, true},
		{"crawl single", "dir", "", 1, modeCrawl, false},
		{"capture many", "dir", "", 3, modeCapture, false},
		{"output conflict", "dir", "out.md", 1, modeStdout, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCrawldownMode(tc.output, tc.outputFile, tc.urls)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got mode %d, want %d", got, tc.want)
			}
		})
	}
}

func newCrawldownTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Page A</title></head><body><main><p>alpha content</p></main></body></html>`)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Page B</title></head><body><main><p>beta content</p></main></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCrawldownCommand_MultipleURLsStdout(t *testing.T) {
	resetCrawldownFlags()
	srv := newCrawldownTestServer(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", srv.URL + "/a", srv.URL + "/b"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("crawldown multi-URL failed: %v", err)
	}

	out := buf.String()
	ia := strings.Index(out, "Page A")
	ib := strings.Index(out, "Page B")
	if ia < 0 || ib < 0 {
		t.Fatalf("expected both pages on stdout, got: %s", out)
	}
	if ia > ib {
		t.Error("expected the pages on stdout in argument order (A then B)")
	}
}

func TestCrawldownCommand_CaptureMultipleURLs(t *testing.T) {
	resetCrawldownFlags()
	srv := newCrawldownTestServer(t)
	dir := t.TempDir()

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	rootCmd.SetArgs([]string{"crawldown", "--output", dir, srv.URL + "/a", srv.URL + "/b"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("crawldown capture failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 captured files, got %d: %v", len(entries), entries)
	}

	seenA, seenB := false, false
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // test reads known temp files
		if err != nil {
			t.Fatalf("ReadFile(%s) error: %v", e.Name(), err)
		}
		if strings.Contains(string(content), "Page A") {
			seenA = true
		}
		if strings.Contains(string(content), "Page B") {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Errorf("expected both pages captured, got A=%v B=%v", seenA, seenB)
	}
}

func TestCrawldownCommand_OutputFileMultipleURLsError(t *testing.T) {
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

	rootCmd.SetErr(new(bytes.Buffer))
	defer rootCmd.SetErr(nil)

	rootCmd.SetArgs([]string{"crawldown", "--output-file", filepath.Join(t.TempDir(), "out.md"), "http://example.com/a", "http://example.com/b"})
	_ = rootCmd.Execute()

	if exited != 1 {
		t.Fatalf("expected exit code 1, got %v", exited)
	}
}

func TestCrawldownCommand_NoFrontmatterAndNoTitle(t *testing.T) {
	resetCmdFlags(rootCmd)
	t.Cleanup(func() { resetCmdFlags(rootCmd) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>My Page</title></head><body><main><h1>Hello</h1><p>World content</p></main></body></html>`)
	}))
	defer srv.Close()

	run := func(extra ...string) string {
		resetCrawldownFlags()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(new(bytes.Buffer))
		defer func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
		}()

		args := append([]string{"crawldown"}, extra...)
		args = append(args, srv.URL)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("crawldown %v failed: %v", extra, err)
		}
		return buf.String()
	}

	out := run("--no-frontmatter")
	if strings.Contains(out, "saved_at:") || strings.HasPrefix(out, "---") {
		t.Errorf("--no-frontmatter still emitted frontmatter:\n%s", out)
	}
	if !strings.Contains(out, "# My Page") {
		t.Errorf("--no-frontmatter should keep the injected title:\n%s", out)
	}

	out = run("--no-title")
	if strings.Contains(out, "# My Page") {
		t.Errorf("--no-title still injected the title:\n%s", out)
	}
	if !strings.Contains(out, "saved_at:") {
		t.Errorf("--no-title should keep the frontmatter:\n%s", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("--no-title lost the body:\n%s", out)
	}
}

func TestCrawldownCommand_FormatReceipts(t *testing.T) {
	resetCmdFlags(rootCmd)
	t.Cleanup(func() { resetCmdFlags(rootCmd) })
	srv := newCrawldownTestServer(t)

	run := func(args ...string) string {
		resetCrawldownFlags()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(new(bytes.Buffer))
		defer func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
		}()

		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("crawldown %v failed: %v", args, err)
		}
		return buf.String()
	}

	t.Run("stdout single json object", func(t *testing.T) {
		out := run("crawldown", "--format", "json", srv.URL+"/a")

		var got map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if got["url"] == "" || got["title"] != "Page A" {
			t.Errorf("unexpected receipt: %v", got)
		}
		if got["markdown"] == nil || got["markdown"] == "" {
			t.Errorf("stdout receipt should carry markdown: %v", got)
		}
		if _, ok := got["path"]; ok {
			t.Errorf("stdout receipt should not carry path: %v", got)
		}
	})

	t.Run("stdout multiple json array", func(t *testing.T) {
		out := run("crawldown", "--format", "json", srv.URL+"/a", srv.URL+"/b")

		var got []map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid JSON array: %v\n%s", err, out)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 receipts, got %d", len(got))
		}
	})

	t.Run("file json receipt has path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.md")
		out := run("crawldown", "--format", "json", "--output-file", path, srv.URL+"/a")

		var got map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if got["path"] != path {
			t.Errorf("expected path %q, got %v", path, got["path"])
		}
		if _, ok := got["markdown"]; ok {
			t.Errorf("file receipt should not carry markdown: %v", got)
		}
	})

	t.Run("stdout single yaml", func(t *testing.T) {
		out := run("crawldown", "--format", "yaml", srv.URL+"/a")
		if !strings.Contains(out, "url:") || !strings.Contains(out, "markdown:") {
			t.Errorf("yaml receipt missing fields:\n%s", out)
		}
	})

	t.Run("crawl json array of paths", func(t *testing.T) {
		dir := t.TempDir()
		out := run("crawldown", "--format", "json", "--output", dir, srv.URL+"/a")

		var got []map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid JSON array: %v\n%s", err, out)
		}
		if len(got) == 0 {
			t.Fatalf("expected at least one crawl receipt, got: %s", out)
		}
		for _, r := range got {
			if r["path"] == nil || r["path"] == "" {
				t.Errorf("crawl receipt should carry a path: %v", r)
			}
			if _, ok := r["markdown"]; ok {
				t.Errorf("crawl receipt should not carry markdown: %v", r)
			}
		}
	})
}

// runCrawldownWithExit runs the command, intercepting exit(), and returns the
// exit code (0 when it was never called) and stdout.
func runCrawldownWithExit(t *testing.T, args ...string) (int, string) {
	t.Helper()
	resetCmdFlags(rootCmd)
	t.Cleanup(func() { resetCmdFlags(rootCmd) })

	out := new(bytes.Buffer)
	rootCmd.SetOut(out)
	rootCmd.SetErr(new(bytes.Buffer))
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	exited := 0
	origExit := exit
	exit = func(code int) {
		exited = code
		panic("exit")
	}
	t.Cleanup(func() { exit = origExit })

	func() {
		defer func() {
			if r := recover(); r != nil && r != "exit" {
				panic(r)
			}
		}()
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
	}()

	return exited, out.String()
}

func TestCrawldownCommand_CrawlOnlyFlagsRejected(t *testing.T) {
	cases := [][]string{
		{"--depth", "2"},
		{"--exclude", "/x"},
		{"--allowed-path", "/x"},
		{"--allowed-path-regex", "^/x"},
		{"--follow-external"},
		{"--ignore-robots-txt"},
	}
	for _, flags := range cases {
		t.Run(strings.TrimPrefix(flags[0], "--"), func(t *testing.T) {
			args := append([]string{"crawldown"}, flags...)
			args = append(args, "http://example.com")

			code, _ := runCrawldownWithExit(t, args...)
			if code != 1 {
				t.Errorf("%v: expected exit 1, got %d", flags, code)
			}
		})
	}

	t.Run("download-docs on stdout", func(t *testing.T) {
		code, _ := runCrawldownWithExit(t, "crawldown", "--download-docs", "http://example.com")
		if code != 1 {
			t.Errorf("expected exit 1, got %d", code)
		}
	})

	t.Run("download-docs with output-file", func(t *testing.T) {
		code, _ := runCrawldownWithExit(t, "crawldown", "--download-docs",
			"--output-file", filepath.Join(t.TempDir(), "out.md"), "http://example.com")
		if code != 1 {
			t.Errorf("expected exit 1, got %d", code)
		}
	})
}

func TestCrawldownCommand_CaptureDownloadDocs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Docs</title></head><body><main><p>See <a href="/doc/sample.pdf">PDF</a></p></main></body></html>`)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Page B</title></head><body><main><p>beta</p></main></body></html>`)
	})
	mux.HandleFunc("/doc/sample.pdf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n%EOF"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	code, _ := runCrawldownWithExit(t, "crawldown", "--output", dir, "--download-docs", srv.URL+"/a", srv.URL+"/b")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	foundPDF, mdCount := false, 0
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".pdf":
			foundPDF = true
		case ".md":
			mdCount++
		}
	}
	if !foundPDF {
		t.Errorf("expected the linked PDF captured in single-page mode, dir: %v", entries)
	}
	if mdCount != 2 {
		t.Errorf("expected 2 captured markdown pages, got %d", mdCount)
	}
}

func TestCrawldownCommand_PartialFailure(t *testing.T) {
	srv := newCrawldownTestServer(t)
	dir := t.TempDir()

	code, _ := runCrawldownWithExit(t, "crawldown", "--output", dir, srv.URL+"/a", srv.URL+"/missing")
	if code != 1 {
		t.Fatalf("expected exit 1 on partial failure, got %d", code)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected the one good page to be written, got %d files: %v", len(entries), entries)
	}

	content, err := os.ReadFile(filepath.Join(dir, entries[0].Name())) //nolint:gosec // test reads known temp files
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !strings.Contains(string(content), "Page A") {
		t.Errorf("expected the good page persisted, got:\n%s", content)
	}
}

func TestCrawldownCommand_Help(t *testing.T) {
	long := crawldownCmd.Long
	for _, want := range []string{
		"stdout", "file (--output-file", "crawl (--output", "capture (--output",
		"Only HTML web pages", "does not abort the rest", "exits non-zero",
		"stderr", "--no-frontmatter", "--no-title", "--delay",
	} {
		if !strings.Contains(long, want) {
			t.Errorf("help text missing %q", want)
		}
	}

	usage := map[string]string{}
	crawldownCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { usage[f.Name] = f.Usage })

	for _, name := range crawlOnlyFlags {
		if !strings.Contains(usage[name], "crawl mode only") {
			t.Errorf("--%s usage should mark it crawl-only, got %q", name, usage[name])
		}
	}

	for _, name := range []string{"no-frontmatter", "no-title"} {
		if !strings.Contains(usage[name], "stdout, file and capture modes") {
			t.Errorf("--%s usage should name the modes it applies to, got %q", name, usage[name])
		}
	}
}

func TestCrawldownCommand_StreamsAndQuiet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Streams</title></head><body><main><h1>Hello</h1><p>World</p></main></body></html>`)
	}))
	defer srv.Close()

	t.Run("content on stdout, progress on stderr", func(t *testing.T) {
		resetCmdFlags(rootCmd)
		t.Cleanup(func() { resetCmdFlags(rootCmd) })
		resetCrawldownFlags()

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		rootCmd.SetOut(outBuf)
		rootCmd.SetErr(errBuf)
		defer func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
		}()

		rootCmd.SetArgs([]string{"crawldown", srv.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("crawldown failed: %v", err)
		}

		if !strings.Contains(outBuf.String(), "Hello") {
			t.Errorf("stdout missing content:\n%s", outBuf.String())
		}
		if strings.Contains(outBuf.String(), "visiting") {
			t.Errorf("stdout polluted with diagnostics:\n%s", outBuf.String())
		}
		if !strings.Contains(errBuf.String(), "visiting") {
			t.Errorf("expected crawl progress on stderr, got:\n%s", errBuf.String())
		}
	})

	t.Run("quiet suppresses info logs", func(t *testing.T) {
		resetCmdFlags(rootCmd)
		t.Cleanup(func() { resetCmdFlags(rootCmd) })
		resetCrawldownFlags()

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		rootCmd.SetOut(outBuf)
		rootCmd.SetErr(errBuf)
		defer func() {
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
		}()

		rootCmd.SetArgs([]string{"crawldown", "--quiet", srv.URL})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("crawldown --quiet failed: %v", err)
		}

		if strings.Contains(errBuf.String(), "visiting") {
			t.Errorf("--quiet should suppress info logs, got:\n%s", errBuf.String())
		}
		if !strings.Contains(outBuf.String(), "Hello") {
			t.Errorf("--quiet dropped stdout content:\n%s", outBuf.String())
		}
	})
}
