package cmd

import (
	"strings"
	"testing"
)

func TestLintChecklistAnchorsDuplicate(t *testing.T) {
	dup := "---\nkind: tasks\n---\n\n## Phase 1\n- [ ] a1 <!-- c1 -->\n\n## Phase 2\n- [ ] b1 <!-- c1 -->\n"
	for _, kind := range []string{ctxTypeTasks, ctxTypePlan, ctxTypeAnalysis, ctxTypeQuestions} {
		issues := lintChecklistAnchors("context/x.md", dup, kind)
		if len(issues) != 1 {
			t.Fatalf("kind %s: issues = %#v, want one", kind, issues)
		}
		if issues[0].Priority != ctxLintWarning {
			t.Fatalf("kind %s: priority = %q, want WARNING", kind, issues[0].Priority)
		}
		if !strings.Contains(issues[0].Message, "duplicate checklist id") {
			t.Fatalf("kind %s: message = %q", kind, issues[0].Message)
		}
	}
}

func TestLintChecklistAnchorsClean(t *testing.T) {
	clean := "---\nkind: plan\n---\n\n## A\n- [ ] a1 <!-- c1 -->\n\n## B\n- [ ] b1 <!-- c2 -->\n"
	if issues := lintChecklistAnchors("context/x.md", clean, ctxTypePlan); len(issues) != 0 {
		t.Fatalf("clean document must yield no issues: %#v", issues)
	}
	// A kind without addressable checklists is skipped.
	if issues := lintChecklistAnchors("context/x.md", clean, ctxTypeDecision); len(issues) != 0 {
		t.Fatalf("non-checklist kind must yield no issues: %#v", issues)
	}
}

func TestLintChecklistAnchorsContinuationDuplicate(t *testing.T) {
	// One anchor on a continuation line: still the item's id, so the duplicate
	// with the item above is detected.
	content := "---\nkind: analysis\n---\n\n## Next steps\n- [ ] a1 <!-- c1 -->\n- [ ] b1\n      continuation <!-- c1 -->\n"
	issues := lintChecklistAnchors("context/x.md", content, ctxTypeAnalysis)
	if len(issues) != 1 {
		t.Fatalf("continuation-anchor duplicate must be detected: %#v", issues)
	}
}
