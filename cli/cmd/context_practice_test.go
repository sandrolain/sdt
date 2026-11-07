package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPracticeTypeRegistered pins the type's shape: it lives under learning/,
// keeps its id bare (`practice/<slug>`) and names its file with the suffix.
func TestPracticeTypeRegistered(t *testing.T) {
	typ, ok := ctxTypeLookup(ctxTypePractice)
	if !ok {
		t.Fatal("practice type is not registered")
	}
	if typ.dir != sdtLearningDir || typ.scheme != ctxSchemeSubpath || typ.suffix != ctxPracticeSuffix {
		t.Errorf("practice type = %+v, want dir %q, subpath scheme, suffix %q", typ, sdtLearningDir, ctxPracticeSuffix)
	}
	if typ.templateFile != "practice.md" {
		t.Errorf("template file = %q, want practice.md", typ.templateFile)
	}
}

// TestPracticeFrontmatterScaffold proves `context new --type practice` writes the
// three fields its contract needs, empty for a draft.
func TestPracticeFrontmatterScaffold(t *testing.T) {
	fm := contextFrontmatter(ctxTypePractice, "A case", "summary", "", "probe", "", "2026-11-07T00:00:00Z", "", "practice/a-case", "", "", "", "", nil, nil, nil)
	for _, want := range []string{"kind: practice", "id: practice/a-case", "concepts: []", `source_span: ""`, `transformation: ""`} {
		if !strings.Contains(fm, want) {
			t.Errorf("scaffold lacks %q:\n%s", want, fm)
		}
	}
}

func practiceDoc(status string, concepts, span, transformation, body string) string {
	return "---\nkind: practice\nstatus: " + status + "\nconcepts: [" + concepts + "]\nsource_span: \"" + span + "\"\ntransformation: \"" + transformation + "\"\n---\n\n" + body
}

// TestLintPractice checks the pointer rule and the draft rule: an active note
// must name a concept the corpus holds, its span and its transformation; a draft
// may be empty and only warns about the missing section.
func TestLintPractice(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join("context", "wiki", "architecture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("context", "wiki", "architecture", "x.md"), []byte("---\nkind: wiki\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := "## Cases\n\n### A case\n\nsituation\n"

	good := practiceDoc("active", "architecture/x", "refs/a.md@abcdef01:1-2", "renamed the company, kept the mechanism", cases)
	if got := lintPractice("p", good, ctxTypePractice); len(got) != 0 {
		t.Errorf("a complete active note should pass, got %+v", got)
	}

	draft := practiceDoc("draft", "", "", "", "")
	got := lintPractice("p", draft, ctxTypePractice)
	if len(got) != 1 || !strings.Contains(got[0].Message, "`## Cases`") {
		t.Errorf("a draft should warn only about the missing section, got %+v", got)
	}

	bad := practiceDoc("active", "architecture/missing", "", "", cases)
	msgs := []string{}
	for _, issue := range lintPractice("p", bad, ctxTypePractice) {
		msgs = append(msgs, issue.Message)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"no page", "source_span", "transformation"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning %q in:\n%s", want, joined)
		}
	}
}
