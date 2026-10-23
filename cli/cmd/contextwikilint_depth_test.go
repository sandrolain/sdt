package cmd

import (
	"strconv"
	"strings"
	"testing"
)

func stubWikiExit(t *testing.T) {
	t.Helper()
	orig := exit
	exit = func(int) {}
	t.Cleanup(func() { exit = orig })
}

func TestWikiLintDepthFlagsThinConceptPage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/architecture/thin.md", firstPageFixture("architecture/thin", "Thin"))
	out := string(execute(t, contextWikiLintCmd, nil))
	if !strings.Contains(out, "density floor") {
		t.Fatalf("a 0-claim page must be flagged:\n%s", out)
	}
	if !strings.Contains(out, "0 claim(s)") || !strings.Contains(out, "0 word(s)") {
		t.Errorf("the finding must name both counts:\n%s", out)
	}
}

func TestWikiLintDepthPassesAtFloor(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	wikiSourceDirs(t)
	page := firstPageFixture("architecture/dense", "Dense") +
		"## Summary\n\n" + strings.Repeat("word ", wikiDepthMinSummaryWords) + "\n\n## Claims\n\n"
	for i := 1; i <= wikiDepthMinClaims; i++ {
		page += "1. {#claim-" + strconv.Itoa(i) + "} a fact (refs/spec.md@abcdef12:1)\n"
	}
	writeCtxDoc(t, "context/wiki/architecture/dense.md", page)
	writeCtxDoc(t, "context/refs/spec.md", "spec\n")
	out := string(execute(t, contextWikiLintCmd, nil))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "architecture/dense.md") && strings.Contains(line, "density floor") {
			t.Fatalf("a page at the floor was flagged:\n%s", line)
		}
	}
}

func TestWikiLintDepthExemptsCoveragePage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	wikiSourceDirs(t)
	page := coveragePage("sources/book", "coverage:\n  units: 1\n  ok: 1\n  part: 0\n  gap: 0\n  no_op: 0\n")
	writeCtxDoc(t, "context/wiki/sources/book.md", page)
	out := string(execute(t, contextWikiLintCmd, nil))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "sources/book.md") && strings.Contains(line, "density floor") {
			t.Fatalf("an index page must be exempt:\n%s", line)
		}
	}
}

func TestWikiSummaryWords(t *testing.T) {
	body := "preamble\n\n## Summary\n\none two three\nmore words\n\n## Claims\n\nnine\n"
	if got := wikiSummaryWords(body); got != 5 {
		t.Errorf("summary words = %d, want 5", got)
	}
	if got := wikiSummaryWords("## Claims\n\nno summary\n"); got != 0 {
		t.Errorf("missing summary = %d, want 0", got)
	}
}
