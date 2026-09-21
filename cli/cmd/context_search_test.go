package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
