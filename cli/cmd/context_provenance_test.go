package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestLintFrontmatterReferencesResolution(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: source\n---\nbody\n")
	noPrio := func(sev string) string { return sev }

	ok := "---\nkind: notes\nsummary: ok\nsources:\n  - analysis/source.md\nlinks:\n  - analysis/source.md\n---\nbody\n"
	if issues := lintFrontmatterReferences("context/notes/ok.md", ok, noPrio); len(issues) != 0 {
		t.Fatalf("expected no issues for a resolvable chain, got %v", issues)
	}

	broken := "---\nkind: notes\nsummary: broken\nsources:\n  - analysis/missing.md\n---\nbody\n"
	issues := lintFrontmatterReferences("context/notes/broken.md", broken, noPrio)
	if len(issues) != 1 {
		t.Fatalf("expected 1 broken source issue, got %d", len(issues))
	}
	if issues[0].Priority != ctxLintWarning || !strings.Contains(issues[0].Message, "broken source reference: analysis/missing.md") {
		t.Errorf("unexpected issue: %s", issues[0].Message)
	}

	relative := "---\nkind: notes\nsummary: rel\nsources:\n  - analysis/source.md\n  - plan/nope.md\n---\nbody\n"
	if issues := lintFrontmatterReferences("context/notes/rel.md", relative, noPrio); len(issues) != 1 {
		t.Fatalf("expected only the unresolvable ref to be flagged, got %v", issues)
	}
}

// TestContextLintProvenanceWarningNonDestructive guards the Phase 3 contract:
// broken sources surface as a WARNING through the lint command while the note
// is never modified.
func TestContextLintProvenanceWarningNonDestructive(t *testing.T) {
	setupContextProject(t)
	note := "---\nkind: notes\nsummary: with provenance\nsources:\n  - analysis/source.md\n  - analysis/missing.md\n---\nbody\n"
	writeCtxDoc(t, "context/notes/keep.md", note)
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: source\n---\nbody\n")

	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken source reference: analysis/missing.md") {
		t.Errorf("expected broken-source WARNING, got:\n%s", out)
	}
	if strings.Contains(string(out), `"CRITICAL"`) {
		t.Errorf("provenance lint must never be CRITICAL:\n%s", out)
	}
	data, err := os.ReadFile("context/notes/keep.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != note {
		t.Error("lint must not modify the note")
	}
}

// TestAgentProvenanceTemplateCoherence guards the provenance contract in the
// notes and wiki instructions: chain shape, evidence references, the lint
// WARNING surface and the never-drop-conflict rule must survive edits.
func TestAgentProvenanceTemplateCoherence(t *testing.T) {
	for _, tc := range []struct {
		name, template string
		wants          []string
	}{
		{"notes", instrNotesTemplate, []string{
			"## Provenance",
			"sources: <optional",
			"via: <optional",
			"source → transformation → derived document → task/worklog",
			"sdt context lint",
			"WARNING",
		}},
		{"wiki", instrWikiTemplate, []string{
			"## Evidence and provenance",
			"refs/<file>@<sha>:<lines>",
			"source → transformation → derived page",
			"never silently drop",
			"supersedes",
		}},
	} {
		for _, want := range tc.wants {
			if !strings.Contains(tc.template, want) {
				t.Errorf("%s: expected %q in template:\n%s", tc.name, want, tc.template)
			}
		}
	}
}
