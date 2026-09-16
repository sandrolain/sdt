package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture creates a file under root with the given content.
func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeCorpus builds a representative knowledge base for handler tests.
func makeCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "context/wiki/backend/auth.md", `---
kind: wiki
title: Auth wiki page
summary: Wiki summary
created: 2026-09-10
---

# wiki body
`)
	writeFixture(t, root, "context/analysis/analy-x.md", `---
kind: analysis
title: "Analysis X"
summary: "Analysis summary"
created: "2026-09-11"
---

analysis body
`)
	writeFixture(t, root, "context/wiki/backlog/blue.md", `---
kind: wiki
title: Blue
created: 2026-09-12
---

blue body
`)
	writeFixture(t, root, "context/board.canvas", `{"nodes":[{"id":"a"}],"edges":[]}`)
	writeFixture(t, root, "context/wiki/topic.map.md", `---
kind: wiki
title: Topic map
created: 2026-09-13
updated: 2026-09-14T09:30:00Z
---

# Topic

- item
`)
	writeFixture(t, root, "context/tmp/scratch.md", `---
kind: wiki
title: Scratch
---

scratch
`)
	writeFixture(t, root, "context/scripts/behind.md", "not a corpus file")
	writeFixture(t, root, "context/refs/clone.md", `---
kind: wiki
title: Refs clone page
---

refs corpus noise
`)
	writeFixture(t, root, "context/../outside.md", `---
kind: wiki
title: Outside root
---

outside the corpus
`)
	writeFixture(t, root, "docs/sdt_tokens.md", `---
kind: notes
title: Outside docs
---

not in the corpus
`)
	writeFixture(t, root, "context/commands/ingest.md", "agent command contract")
	writeFixture(t, root, "context/instructions/project.md", "agent instructions")
	writeFixture(t, root, "context/sdtdocs/README.md", "generated per-command reference")
	writeFixture(t, root, "context/README.md", "context umbrella readme")
	return root
}

func TestResolveRootExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveRoot(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("got %q want %q", got, dir)
	}
}

func TestResolveRootMissingFlag(t *testing.T) {
	if _, err := resolveRoot(filepath.Join(t.TempDir(), "nope"), ""); err == nil {
		t.Error("expected error for missing --root path")
	}
}

func TestResolveRootByConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, sdtConfigFile), []byte("project: p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveRoot("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("got %q want %q", got, dir)
	}
}

func TestResolveRootWalkUp(t *testing.T) {
	proj := t.TempDir()
	if err := os.WriteFile(filepath.Join(proj, sdtConfigFile), []byte("project: p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(proj) // os.Getwd() resolves /var → /private/var
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(proj, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	got, err := resolveRoot("", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != canonical {
		t.Errorf("got %q want %q", got, canonical)
	}
}

func TestResolveRootBothAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolveRoot("", dir); err == nil {
		t.Error("expected error when no .sdt.yaml and no --root")
	}
}

func TestNewHandler(t *testing.T) {
	h, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Error("expected non-nil handler")
	}
}

func TestNewHandlerMissingRoot(t *testing.T) {
	if _, err := newHandler(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected error for missing root")
	}
}

func TestNewHandlerNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newHandler(f); err == nil {
		t.Error("expected error for non-directory root")
	}
}

func TestRootCmdHelp(t *testing.T) {
	root := newRootCmd()
	if root.Use != "sdtviewer" || len(root.Commands()) != 1 {
		t.Errorf("root = %+v, commands = %d", root, len(root.Commands()))
	}
	root.SetArgs([]string{"serve", "--help"})
	if err := root.Execute(); err != nil {
		t.Errorf("serve --help failed: %v", err)
	}
	if err := run(); err != nil {
		t.Errorf("run failed: %v", err)
	}
}

func TestServeCommandRuns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, sdtConfigFile), []byte("project: p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origListen := listen
	origOpen := openBrowser
	listen = func(addr string, h http.Handler) error {
		if addr != "127.0.0.1:8443" {
			t.Errorf("addr = %q", addr)
		}
		if h == nil {
			t.Error("handler is nil")
		}
		return nil
	}
	openBrowser = func(url string) error { return nil }
	defer func() { listen, openBrowser = origListen, origOpen }()
	root := newRootCmd()
	root.SetArgs([]string{"serve", "--root", dir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestServeCommandNoConfig(t *testing.T) {
	// serve without .sdt.yaml reachable and without a valid --root must error.
	root := newRootCmd()
	root.SetArgs([]string{"serve", "--root", filepath.Join(t.TempDir(), "missing")})
	if err := root.Execute(); err == nil {
		t.Error("expected error")
	}
}

func TestResolveRootNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRoot(f, ""); err == nil {
		t.Error("expected error for non-directory --root")
	}
}

func TestServeFlagDefaults(t *testing.T) {
	cmd := newServeCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	host, err := cmd.Flags().GetString("host")
	if err != nil {
		t.Fatal(err)
	}
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		t.Fatal(err)
	}
	noOpen, err := cmd.Flags().GetBool("no-open")
	if err != nil {
		t.Fatal(err)
	}
	root, err := cmd.Flags().GetString("root")
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" || port != 8443 || noOpen || root != "" {
		t.Errorf("defaults: host=%q port=%d noOpen=%v root=%q", host, port, noOpen, root)
	}
}

func TestBrowserCmd(t *testing.T) {
	url := "http://127.0.0.1:8443"
	if c, a := browserCmd("windows", url); c != "rundll32" || len(a) != 2 {
		t.Errorf("windows: got %q %v", c, a)
	}
	if c, a := browserCmd("darwin", url); c != "open" || len(a) != 1 {
		t.Errorf("darwin: got %q %v", c, a)
	}
	if c, a := browserCmd("linux", url); c != "xdg-open" || len(a) != 1 {
		t.Errorf("linux: got %q %v", c, a)
	}
}

func TestOpenBrowser(t *testing.T) {
	var gotName string
	var gotArgs []string
	origRun := runCmd
	runCmd = func(name string, args ...string) error {
		gotName, gotArgs = name, args
		return nil
	}
	defer func() { runCmd = origRun }()
	if err := openBrowser("http://127.0.0.1:8443"); err != nil {
		t.Fatal(err)
	}
	if gotName != "open" && gotName != "xdg-open" && gotName != "rundll32" {
		t.Errorf("unexpected launcher %q args %v", gotName, gotArgs)
	}
}

func TestServeError(t *testing.T) {
	// no .sdt.yaml anywhere reachable and no --root → clear error
	if err := serve("127.0.0.1", 1, true, "/does/not/exist", func(string, http.Handler) error { return nil }); err == nil {
		t.Error("expected error for bad root")
	}
}

func TestServeStarts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, sdtConfigFile), []byte("project: p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotAddr string
	origOpen := openBrowser
	openBrowser = func(url string) error { return nil }
	defer func() { openBrowser = origOpen }()
	err := serve("127.0.0.1", 38443, false, dir, func(addr string, h http.Handler) error {
		gotAddr = addr
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAddr != "127.0.0.1:38443" {
		t.Errorf("addr = %q", gotAddr)
	}
}

func TestTreeOutput(t *testing.T) {
	root := makeCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Entries []treeEntry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]treeEntry{}
	for _, e := range out.Entries {
		byPath[e.Path] = e
	}
	// wiki page: kind + frontmatter fields
	if e, ok := byPath["context/wiki/backend/auth.md"]; !ok {
		t.Errorf("missing context/wiki/backend/auth.md in %v", byPath)
	} else if e.Kind != "wiki" || e.Title != "Auth wiki page" || e.Summary != "Wiki summary" || e.Created != "2026-09-10" {
		t.Errorf("wiki entry wrong: %+v", e)
	} else if e.Modified == "" {
		t.Errorf("wiki entry missing modified mtime fallback: %+v", e)
	}
	// analysis page
	if e, ok := byPath["context/analysis/analy-x.md"]; !ok {
		t.Errorf("missing context/analysis/analy-x.md")
	} else if e.Kind != "analysis" {
		t.Errorf("analysis kind wrong: %+v", e)
	}
	// canvas tagged
	if e, ok := byPath["context/board.canvas"]; !ok {
		t.Errorf("missing context/board.canvas in %v", byPath)
	} else if !e.Canvas || e.Kind != "canvas" || e.Title != "board" {
		t.Errorf("canvas entry wrong: %+v", e)
	} else if e.Modified == "" {
		t.Errorf("canvas entry missing modified mtime: %+v", e)
	}
	// map document flagged with a canonical mapId; plain docs are not
	if e, ok := byPath["context/wiki/topic.map.md"]; !ok {
		t.Errorf("missing context/wiki/topic.map.md in %v", byPath)
	} else if !e.IsMap || e.MapID != "topic.map" {
		t.Errorf("map entry wrong: %+v", e)
	} else if e.Modified != "2026-09-14T09:30:00Z" {
		t.Errorf("map modified = %q, want frontmatter updated", e.Modified)
	}
	if e, ok := byPath["context/wiki/backend/auth.md"]; ok && (e.IsMap || e.MapID != "") {
		t.Errorf("plain doc flagged as map: %+v", e)
	}
	// corpus exclusions (shared set) plus anything outside the corpus.
	for _, p := range []string{
		"context/tmp/scratch.md",
		"context/scripts/behind.md",
		"context/refs/clone.md",
		"context/commands/ingest.md",
		"context/instructions/project.md",
		"context/sdtdocs/README.md",
		"context/README.md",
		"docs/sdt_tokens.md",
	} {
		if _, ok := byPath[p]; ok {
			t.Errorf("excluded path present: %s", p)
		}
	}
	if _, ok := byPath["../outside.md"]; ok {
		t.Errorf("outside-corpus path present: ../outside.md")
	}
	if len(out.Entries) != 5 {
		t.Errorf("expected 5 entries, got %d: %v", len(out.Entries), out.Entries)
	}
}

func TestDocMarkdown(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("context/wiki/backend/auth.md"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out docResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Path != "context/wiki/backend/auth.md" {
		t.Errorf("path = %q", out.Path)
	}
	if !strings.Contains(out.Frontmatter, "kind: wiki") {
		t.Errorf("frontmatter missing kind: %q", out.Frontmatter)
	}
	if !strings.Contains(out.Markdown, "# wiki body") {
		t.Errorf("markdown missing body: %q", out.Markdown)
	}
}

func TestDocCanvas(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("context/board.canvas"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out canvasResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Path != "context/board.canvas" {
		t.Errorf("path = %q", out.Path)
	}
	var raw map[string]any
	if err := json.Unmarshal(out.Canvas, &raw); err != nil {
		t.Fatalf("canvas not raw JSON: %v", err)
	}
	if raw["nodes"] == nil {
		t.Errorf("canvas nodes missing: %v", raw)
	}
}

func TestDocMissing(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("nope.md"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDocExcludedCorpus(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	for _, path := range []string{
		"context/commands/ingest.md",
		"context/instructions/project.md",
		"context/sdtdocs/README.md",
		"context/README.md",
		"context/refs/clone.md",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape(path), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestDocTraversal(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	for _, path := range []string{"../secret.md", "../../etc/passwd", "/etc/passwd", "a/../../x.md"} {
		req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape(path), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestDocUnsupportedExt(t *testing.T) {
	root := makeCorpus(t)
	writeFixture(t, root, "context/notes/note.txt", "text")
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("context/notes/note.txt"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDocOutsideCorpus(t *testing.T) {
	root := makeCorpus(t)
	writeFixture(t, root, "docs/secret.md", `---
kind: notes
title: Secret
---

secret
`)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("docs/secret.md"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for outside-corpus doc", rec.Code)
	}
}

func TestDocDirectory(t *testing.T) {
	root := makeCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("context/wiki"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestIndexPlaceholder(t *testing.T) {
	// No embedded SPA: the placeholder handler serves / only.
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.handleIndex(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "sdtviewer") {
		t.Errorf("index missing title: %s", body)
	}
}

func TestIndexNotFound(t *testing.T) {
	// Without an embedded SPA, non-root paths 404.
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	rec := httptest.NewRecorder()
	s.handleIndex(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestTreeUnreadable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "context")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "broken.md")
	if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDocUnreadable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "context")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "broken.md")
	if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+url.QueryEscape("context/broken.md"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWriteJSONEncodeError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusInternalServerError, make(chan int))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestTreeQuotedFrontmatter(t *testing.T) {
	root := makeCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tree", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out struct {
		Entries []treeEntry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, e := range out.Entries {
		if e.Path != "context/analysis/analy-x.md" {
			continue
		}
		if e.Title != "Analysis X" || e.Summary != "Analysis summary" || e.Created != "2026-09-11" {
			t.Errorf("quoted frontmatter not normalized: %+v", e)
		}
		return
	}
	t.Error("analysis entry not found")
}
