package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lintQuestionDoc runs lintDoc on a questions-kind fixture and returns the
// findings, so the deferred-reason and evidence checks are judged in isolation.
func lintQuestionDoc(t *testing.T, frontmatter string) []ctxLintIssue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "q.md")
	content := "---\nkind: questions\nsummary: s\n" + frontmatter + "---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return decorateLintHints(lintDoc(path))
}

func TestLintQuestionsDeferredReason(t *testing.T) {
	cases := []struct {
		name, frontmatter string
		want              int
	}{
		{"deferred without reason warns", "status: deferred\n", 1},
		{"deferred with reason passes", "status: deferred\ndeferred_reason: waiting on vendor X\n", 0},
		{"investigating is not deferred", "status: investigating\n", 0},
		{"blocked is not deferred", "status: blocked\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got int
			for _, it := range lintQuestionDoc(t, c.frontmatter) {
				if strings.Contains(it.Message, "deferred_reason") {
					got++
					if it.Priority != ctxLintSuggestion {
						t.Errorf("deferred-reason finding must be SUGGESTION, got %s", it.Priority)
					}
					if it.Hint == "" {
						t.Error("deferred-reason finding must carry a remediation hint")
					}
				}
			}
			if got != c.want {
				t.Fatalf("deferred-reason findings = %d, want %d", got, c.want)
			}
		})
	}
}

func TestLintQuestionsEvidenceReference(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/base.md", "---\nkind: analysis\nsummary: base\nstatus: active\n---\n")
	writeCtxDoc(t, "context/questions/ok.md",
		"---\nkind: questions\nsummary: q\nstatus: active\nevidence:\n  - analysis/base.md\n---\n")
	writeCtxDoc(t, "context/questions/bad.md",
		"---\nkind: questions\nsummary: q\nstatus: active\nevidence:\n  - analysis/missing.md\n---\n")

	for _, it := range decorateLintHints(lintDoc("context/questions/ok.md")) {
		if strings.Contains(it.Message, "evidence reference") {
			t.Fatalf("resolvable evidence reference must not warn: %#v", it)
		}
	}
	var broken int
	for _, it := range decorateLintHints(lintDoc("context/questions/bad.md")) {
		if strings.Contains(it.Message, "broken evidence reference") {
			broken++
			if it.Hint == "" {
				t.Error("broken evidence reference must carry a remediation hint")
			}
		}
	}
	if broken != 1 {
		t.Fatalf("broken evidence reference findings = %d, want 1", broken)
	}
}

// lintKindDoc runs lintDoc on a fixture of the given kind with extra
// frontmatter lines, hints decorated.
func lintKindDoc(t *testing.T, kind, frontmatter string) []ctxLintIssue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "d.md")
	content := "---\nkind: " + kind + "\nsummary: s\n" + frontmatter + "---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return decorateLintHints(lintDoc(path))
}

func TestLintEvidenceClass(t *testing.T) {
	cases := []struct {
		name, kind, frontmatter string
		want                    int
	}{
		{"valid fact on analysis", ctxTypeAnalysis, "status: active\nevidence_class: fact\n", 0},
		{"valid decision on decision", ctxTypeDecision, "evidence_class: decision\n", 0},
		{"absent key is silent", ctxTypeAnalysis, "status: active\n", 0},
		{"invalid value warns", ctxTypeAnalysis, "status: active\nevidence_class: guessed\n", 1},
		{"other kind ignored", ctxTypeNotes, "evidence_class: guessed\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got int
			for _, it := range lintKindDoc(t, c.kind, c.frontmatter) {
				if strings.Contains(it.Message, "undeclared frontmatter key `evidence_class`") {
					t.Fatalf("evidence_class must be a declared key: %#v", it)
				}
				if !strings.Contains(it.Message, "`evidence_class`") {
					continue
				}
				got++
				if it.Priority != ctxLintWarning {
					t.Errorf("evidence_class finding must be WARNING, got %s", it.Priority)
				}
				if it.Hint == "" {
					t.Error("evidence_class finding must carry a remediation hint")
				}
			}
			if got != c.want {
				t.Fatalf("evidence_class findings = %d, want %d", got, c.want)
			}
		})
	}
}

func TestQuestionsStatusVocabulary(t *testing.T) {
	tp, ok := ctxTypeLookup(ctxTypeQuestions)
	if !ok {
		t.Fatal("questions kind missing from the registry")
	}
	for _, st := range []string{taskFileStatusLegacy, questionStatusInvestigating, questionStatusBlocked, questionStatusDeferred, questionStatusResolved} {
		if !ctxStatusInVocab(tp, st) {
			t.Errorf("questions vocabulary must accept %q", st)
		}
	}
	if ctxStatusInVocab(tp, "bogus") {
		t.Error("questions vocabulary must reject an unknown status")
	}
}
