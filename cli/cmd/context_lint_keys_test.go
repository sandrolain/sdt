package cmd

import "testing"

func TestLintUndeclaredKeysClean(t *testing.T) {
	content := "---\nkind: analysis\nuid: 01a0eba1-c6cd-7e9b-8754-a8020ce74450\ntitle: T\nsummary: S\nstatus: active\ncreated: 2026-09-29T05:27:23Z\nupdated: 2026-09-29T05:27:23Z\nlinks:\n---\nbody\n"
	if got := lintUndeclaredFrontmatterKeys("x.md", content); len(got) != 0 {
		t.Errorf("declared keys flagged: %+v", got)
	}
}

func TestLintUndeclaredKeysSuggestion(t *testing.T) {
	content := "---\nkind: analysis\nsummary: S\nmy_custom_field: x\nanother: y\n---\nbody\n"
	got := lintUndeclaredFrontmatterKeys("x.md", content)
	if len(got) != 2 {
		t.Fatalf("issues = %+v", got)
	}
	for _, iss := range got {
		if iss.Priority != ctxLintSuggestion {
			t.Errorf("priority = %s, want SUGGESTION", iss.Priority)
		}
	}
	// Sorted alphabetically.
	if got[0].Message >= got[1].Message {
		t.Errorf("not sorted: %q then %q", got[0].Message, got[1].Message)
	}
}

func TestLintCLIOwnedKeysDeclared(t *testing.T) {
	content := "---\nkind: plan\nuid: u\nanalysis_id: a\nplan_id: p\nsummary: S\n---\nbody\n"
	if got := lintUndeclaredFrontmatterKeys("x.md", content); len(got) != 0 {
		t.Errorf("CLI-owned keys flagged: %+v", got)
	}
}

func TestLintUndeclaredKeysNoFrontmatter(t *testing.T) {
	if got := lintUndeclaredFrontmatterKeys("x.md", "no frontmatter\n"); len(got) != 0 {
		t.Errorf("issues = %+v", got)
	}
}
