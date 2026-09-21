package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPriorArtQuery(t *testing.T) {
	if got := priorArtQuery("search", "Rank fusion", []string{"bleve", "rrf"}); got != "Rank fusion search bleve rrf" {
		t.Errorf("priorArtQuery = %q", got)
	}
	if got := priorArtQuery("", "", nil); got != "" {
		t.Errorf("empty query expected, got %q", got)
	}
}

func TestPriorArtSectionAndLinks(t *testing.T) {
	hits := []priorArtHit{
		{Path: "context/analysis/a.md", Kind: "analysis", Status: "active", Title: "Hybrid", Summary: "Fuse BM25. With more text after.", Score: 0.5},
		{Path: "context/plan/b.md", Kind: "plan", Summary: "", Score: 0.2},
	}
	sec := priorArtSection(hits)
	for _, want := range []string{"## Prior art", "[[context/analysis/a.md]]", "(analysis/active, score 0.50)", "Fuse BM25.", "(no summary)"} {
		if !strings.Contains(sec, want) {
			t.Errorf("prior-art section missing %q:\n%s", want, sec)
		}
	}
	if priorArtSection(nil) != "" {
		t.Error("empty hits must render no section")
	}
	links := priorArtLinks(hits, 5)
	if len(links) != 2 || links[0] != "analysis/a" || links[1] != "plan/b" {
		t.Errorf("priorArtLinks = %v", links)
	}
	if got := priorArtLinks(hits, 1); len(got) != 1 {
		t.Errorf("max cap not honored: %v", got)
	}
}

func TestInjectLinks(t *testing.T) {
	base := "---\nkind: analysis\nsummary: s\n---\n\nbody\n"
	got := injectLinks(base, []string{"analysis/a", "analysis/b"})
	if !strings.Contains(got, "links:\n  - analysis/a\n  - analysis/b\n") {
		t.Errorf("links not injected:\n%s", got)
	}
	// An existing links block is never rewritten.
	withLinks := "---\nkind: analysis\nsummary: s\nlinks:\n  - analysis/keep\n---\n\nbody\n"
	if out := injectLinks(withLinks, []string{"analysis/a"}); out != withLinks {
		t.Errorf("existing links must be preserved:\n%s", out)
	}
	// Content without frontmatter is returned unchanged.
	if out := injectLinks("no frontmatter", []string{"x"}); out != "no frontmatter" {
		t.Errorf("unexpected change: %q", out)
	}
}

func TestContextNewPriorArtPrefill(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	// Reset the shared index cache so the search reads this temp corpus.
	ctxSearchIndex = nil
	writeCtxDoc(t, "context/analysis/hybrid.md", "---\nkind: analysis\nsummary: reciprocal rank fusion of lexical and semantic rankings\nobjective: search\n---\n\n## Design\n\nFuse BM25 with static embeddings.\n")

	out := string(execute(t, contextNewCmd, nil, "--type", "analysis", "--title", "Rank fusion for retrieval",
		"--objective", "search", "--prior-art", "--prior-art-links", "--summary", "s"))
	path := strings.TrimSpace(out)
	content := mustReadFile(t, path)
	if !strings.Contains(content, "## Prior art") || !strings.Contains(content, "context/analysis/hybrid.md") {
		t.Errorf("expected prior-art candidates in the body:\n%s", content)
	}
	if !strings.Contains(content, "links:\n  - analysis/hybrid\n") {
		t.Errorf("expected pre-filled links:\n%s", content)
	}
	if _, err := filepath.Abs(path); err != nil {
		t.Fatal(err)
	}
	_ = dir
	ctxSearchIndex = nil
}
