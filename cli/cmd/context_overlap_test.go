package cmd

import (
	"strings"
	"testing"
)

func TestLintAnalysisRelationsExplicitChoice(t *testing.T) {
	noRel := "context/analysis/x.md"
	none := "---\nkind: analysis\nsummary: s\nlinks: none\n---\nbody\n"
	linked := "---\nkind: analysis\nsummary: s\nlinks:\n  - analysis/other.md\n---\nbody\n"
	superseded := "---\nkind: analysis\nsummary: s\nsupersedes:\n  - analysis/old.md\n---\nbody\n"
	other := "---\nkind: plan\nsummary: s\n---\nbody\n"

	has := func(content, kind string) bool {
		for _, it := range lintAnalysisRelations(noRel, content, kind, func(s string) string { return s }) {
			if strings.Contains(it.Message, "declares no relation") {
				return true
			}
		}
		return false
	}
	if !has("---\nkind: analysis\nsummary: s\n---\nbody\n", ctxTypeAnalysis) {
		t.Error("expected a no-relation SUGGESTION for a bare analysis")
	}
	if has(none, ctxTypeAnalysis) {
		t.Error("links: none should opt out")
	}
	if has(linked, ctxTypeAnalysis) {
		t.Error("a links reference should satisfy the rule")
	}
	if has(superseded, ctxTypeAnalysis) {
		t.Error("a supersedes reference should satisfy the rule")
	}
	if has(other, ctxTypePlan) {
		t.Error("the rule must apply to analyses only")
	}
}

func TestLintOverlappingAnalyses(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md",
		"---\nkind: analysis\nsummary: Hybrid search with bleve and static embeddings\nobjective: obj-y\ntitle: Hybrid search design\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/b.md",
		"---\nkind: analysis\nsummary: Hybrid search with bleve and static embeddings design\nobjective: obj-y\ntitle: Hybrid search design\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/c.md",
		"---\nkind: analysis\nsummary: Totally different topic about graph layouts\nobjective: obj-y\ntitle: Graph layouts\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/d.md",
		"---\nkind: analysis\nsummary: Hybrid search with bleve and static embeddings\nobjective: obj-z\ntitle: Hybrid search design\n---\nbody\n")

	issues := lintOverlappingAnalyses([]string{
		"context/analysis/a.md", "context/analysis/b.md", "context/analysis/c.md", "context/analysis/d.md",
	})
	found := false
	for _, it := range issues {
		if strings.Contains(it.Message, "overlaps analysis") {
			found = true
			if it.Priority != ctxLintSuggestion {
				t.Errorf("expected SUGGESTION, got %s", it.Priority)
			}
		}
		if strings.Contains(it.Path, "c.md") || strings.Contains(it.Path, "d.md") {
			t.Errorf("c.md (different topic) and d.md (different objective) must not be reported: %s", it.Message)
		}
	}
	if !found {
		t.Errorf("expected an overlap SUGGESTION for a.md/b.md, got %v", issues)
	}
}
