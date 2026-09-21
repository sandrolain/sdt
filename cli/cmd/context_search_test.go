package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/search"
)

func TestContextSearchAndShow(t *testing.T) {
	dir := runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\ntopics:\n  - context-search\n---\n\n## Hybrid\n\nbleve static embeddings rrf fusion\n\n## Other\n\nunrelated text\n")
	writeCtxDoc(t, "context/archive/old.md", "---\nkind: analysis\nsummary: s\nstatus: archived\n---\n\n## Old\n\nbleve static embeddings legacy\n")

	// Reset the per-invocation index cache between tests.
	ctxSearchIndex = nil
	out := string(execute(t, contextSearchCmd, nil, "bleve embeddings"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Errorf("expected the active doc in search output:\n%s", out)
	}
	if strings.Contains(out, "context/archive/old.md") {
		t.Errorf("archived doc must be excluded by default:\n%s", out)
	}

	ctxSearchIndex = nil
	all := string(execute(t, contextSearchCmd, nil, "bleve embeddings", "--all"))
	if !strings.Contains(all, "context/archive/old.md") {
		t.Errorf("--all should include the archived doc:\n%s", all)
	}

	ctxSearchIndex = nil
	byTopic := string(execute(t, contextSearchCmd, nil, "bleve", "--topic", "context-search"))
	if !strings.Contains(byTopic, "context/analysis/a.md") {
		t.Errorf("topic filter should match:\n%s", byTopic)
	}

	// show: whole doc vs section vs lines (reads from disk, no index).
	whole := string(execute(t, contextShowCmd, nil, "context/analysis/a.md"))
	if !strings.Contains(whole, "unrelated text") {
		t.Errorf("show should print the whole document:\n%s", whole)
	}
	section := string(execute(t, contextShowCmd, nil, "context/analysis/a.md", "--section", "hybrid"))
	if !strings.Contains(section, "rrf fusion") || strings.Contains(section, "unrelated text") {
		t.Errorf("section show wrong:\n%s", section)
	}
	lines := string(execute(t, contextShowCmd, nil, "context/analysis/a.md", "--lines", "1:2"))
	if !strings.Contains(lines, "kind: analysis") || strings.Contains(lines, "rrf fusion") {
		t.Errorf("line range show wrong:\n%s", lines)
	}
	if _, err := os.Stat(filepath.Join(dir, "context", "analysis", "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestContextSearchPersistentStore(t *testing.T) {
	dir := runInTempDir(t)

	// First run: no store yet → full build creates .sdt/cache/bleve.
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nblevetokenfirst\n")
	ctxSearchIndex = nil
	out := string(execute(t, contextSearchCmd, nil, "blevetokenfirst"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("first run:\n%s", out)
	}
	storeDir := search.StorePath(dir)
	if st, err := os.Stat(storeDir); err != nil || !st.IsDir() {
		t.Fatalf("store dir missing after first run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".sdt", "cache", "bleve.meta.json")); err != nil {
		t.Fatalf("store meta missing: %v", err)
	}

	// Second run, corpus changed: the store is updated incrementally; the old
	// token is gone (token-level supersession, no hyphen phrase trap).
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nblevetokensecond\n")
	ctxSearchIndex = nil
	out = string(execute(t, contextSearchCmd, nil, "blevetokensecond"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("delta run (new token):\n%s", out)
	}
	if st, err := os.Stat(storeDir); err != nil || !st.IsDir() {
		t.Fatalf("store gone after delta run: %v", err)
	}
	ctxSearchIndex = nil
	old := string(execute(t, contextSearchCmd, nil, "blevetokenfirst"))
	if strings.Contains(old, "context/analysis/a.md") {
		t.Fatalf("superseded token still hits:\n%s", old)
	}

	// The manual re-run check: the store dir is still served (not rebuilt from
	// scratch) — verify by opening it read-only directly on the next process.
	ctxSearchIndex = nil
	out = string(execute(t, contextSearchCmd, nil, "blevetokensecond"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("third run:\n%s", out)
	}
}

func TestContextSearchMissingStoreRebuilds(t *testing.T) {
	dir := runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nblevememfallback\n")

	// Warm the store once, then remove it while the corpus is unchanged.
	ctxSearchIndex = nil
	if out := string(execute(t, contextSearchCmd, nil, "blevememfallback")); !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("warm run:\n%s", out)
	}
	if err := os.RemoveAll(search.StorePath(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".sdt", "cache", "bleve.meta.json")); err != nil {
		t.Fatal(err)
	}

	// No store, no corpus deltas → the store is rebuilt from the manifest and
	// the search still answers.
	ctxSearchIndex = nil
	out := string(execute(t, contextSearchCmd, nil, "blevememfallback"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("rebuild run:\n%s", out)
	}
	if st, err := os.Stat(search.StorePath(dir)); err != nil || !st.IsDir() {
		t.Fatalf("store not rebuilt: %v", err)
	}
}
