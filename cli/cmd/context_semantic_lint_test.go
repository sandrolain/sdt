package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/semantic"
)

func TestSemanticDuplicatesFlagsMissedPair(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: a\n---\nalpha one two three\n")
	writeCtxDoc(t, "context/notes/b.md", "---\nkind: notes\nsummary: b\n---\ncompletely different words here\n")
	snap := &semantic.Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#body": {1, 0},
		"context/notes/b.md#body": {0.99, 0.01},
	}}
	issues := lintSemanticDuplicates([]string{"context/notes/a.md", "context/notes/b.md"}, snap)
	if len(issues) != 1 || issues[0].Path != "context/notes/b.md" || !strings.Contains(issues[0].Message, "semantically near-duplicate") {
		t.Fatalf("issues = %#v, want one semantic near-duplicate on b", issues)
	}
	if issues[0].Priority != ctxLintSuggestion {
		t.Fatalf("priority = %q, want SUGGESTION", issues[0].Priority)
	}
}

func TestSemanticDuplicatesSkipsLexicalPair(t *testing.T) {
	runInTempDir(t)
	body := "same words repeated over and over here\n"
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: a\n---\n"+body)
	writeCtxDoc(t, "context/notes/b.md", "---\nkind: notes\nsummary: b\n---\n"+body)
	snap := &semantic.Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#body": {1, 0},
		"context/notes/b.md#body": {1, 0},
	}}
	if issues := lintSemanticDuplicates([]string{"context/notes/a.md", "context/notes/b.md"}, snap); len(issues) != 0 {
		t.Fatalf("issues = %#v, want none (the lexical pass already flags it)", issues)
	}
}

func TestSemanticLintNoSnapshot(t *testing.T) {
	runInTempDir(t)
	if issues := lintSemanticDuplicates([]string{"context/notes/a.md"}, nil); issues != nil {
		t.Fatalf("issues = %#v, want nil", issues)
	}
	if issues := lintSemanticOverlaps(nil, &semantic.Snapshot{}); issues != nil {
		t.Fatalf("issues = %#v, want nil", issues)
	}
}

func TestSemanticOverlapsFlagsSameObjective(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nobjective: obj\nsummary: a\nstatus: active\n---\nalpha\n")
	writeCtxDoc(t, "context/analysis/b.md", "---\nkind: analysis\nobjective: obj\nsummary: b\nstatus: active\n---\nbeta\n")
	snap := &semantic.Snapshot{SectionVectors: map[string][]float32{
		"context/analysis/a.md#body": {1, 0},
		"context/analysis/b.md#body": {0.99, 0.01},
	}}
	issues := lintSemanticOverlaps([]string{"context/analysis/a.md", "context/analysis/b.md"}, snap)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "semantically overlaps") {
		t.Fatalf("issues = %#v, want one semantic overlap", issues)
	}
}
