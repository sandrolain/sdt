package cmd

import (
	"strings"
	"testing"
)

const learningGoodPage = `---
kind: lesson
id: architecture/four-key-metrics
type: concept
objectives:
  - "name the four metrics and the pipeline point each is measured at"
  - "calculate change failure rate from failures over deploys"
difficulty: intermediate
minutes: 12
---

## Summary

x
`

// TestLearningFamilyIsRegistered proves the family is declared in one place: the
// register the `.sdt.yaml` key is validated against.
func TestLearningFamilyIsRegistered(t *testing.T) {
	fam, ok := frontmatterFamilyBySlug(ctxFrontmatterLearning)
	if !ok {
		t.Fatal("learning is not a registered frontmatter family")
	}
	for _, key := range []string{"objectives", "difficulty", "minutes"} {
		if !containsItem(fam.Keys, key) {
			t.Errorf("family keys = %v, want %q", fam.Keys, key)
		}
	}
}

// TestLearningBlockAcceptsAShapedBlock is the green case for the page block.
func TestLessonLearningMetadataAcceptsACompleteShape(t *testing.T) {
	issues := lintLearningBlock("context/learning/x.md", learningGoodPage, ctxTypeLesson)
	for _, i := range issues {
		if i.Priority == ctxLintWarning {
			t.Errorf("a well-shaped block produced a WARNING: %s", i.Message)
		}
	}
}

func TestWikiLearningMetadataIsRejected(t *testing.T) {
	page := "---\nkind: wiki\nid: a/b\nlearning: {objectives: [do thing]}\n---\n\n## Summary\n\nx\n"
	issues := lintLearningBlock("context/wiki/x.md", page, ctxTypeWiki)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "obsolete") {
		t.Fatalf("legacy wiki metadata was not rejected: %+v", issues)
	}
}

// TestLearningBlockOtherKindsAreIgnored keeps the check on concept pages.
func TestLearningBlockOtherKindsAreIgnored(t *testing.T) {
	if issues := lintLearningBlock("context/notes/x.md", learningGoodPage, ctxTypeNotes); len(issues) != 0 {
		t.Errorf("a notes entry was checked as a concept page: %+v", issues)
	}
}

// TestLearningBlockRejectsBadValues covers each rule the block carries, so a
// malformed block names its defect rather than being silently ignored.
func TestLearningBlockRejectsBadValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"one objective", func(s string) string {
			return strings.Replace(s, "  - \"calculate change failure rate from failures over deploys\"\n", "", 1)
		}, "want 2-4"},
		{"five objectives", func(s string) string {
			return strings.Replace(s, "difficulty:", "  - \"apply the reporting window rule\"\n  - \"choose a dashboard audience\"\n  - \"justify weekly discussion\"\ndifficulty:", 1)
		}, "want 2-4"},
		{"duplicate objective", func(s string) string {
			return strings.Replace(s, "  - \"calculate change failure rate from failures over deploys\"", "  - \"name the four metrics and the pipeline point each is measured at\"", 1)
		}, "repeats"},
		{"bad difficulty", func(s string) string {
			return strings.Replace(s, "difficulty: intermediate", "difficulty: expert", 1)
		}, "not one of"},
		{"missing difficulty", func(s string) string {
			return strings.Replace(s, "difficulty: intermediate", "", 1)
		}, "difficulty` is missing"},
		{"minutes out of range", func(s string) string {
			return strings.Replace(s, "minutes: 12", "minutes: 900", 1)
		}, "outside 5-60"},
		{"minutes not a number", func(s string) string {
			return strings.Replace(s, "minutes: 12", "minutes: twelve", 1)
		}, "must be a number"},
		{"objectives not a list", func(s string) string {
			return strings.Replace(s, "objectives:\n  -", "objectives: none\n# -", 1)
		}, "must list 2-4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := lintLearningBlock("context/learning/x.md", tc.mutate(learningGoodPage), ctxTypeLesson)
			joined := ""
			for _, i := range issues {
				joined += i.Message + "\n"
			}
			if !strings.Contains(joined, tc.want) {
				t.Errorf("findings do not mention %q:\n%s", tc.want, joined)
			}
		})
	}
}

// TestLearningObjectiveVerbIsAdvisory proves an unknown opening verb is visible
// without being refused: the verb set is a suggestion, not a closed gate.
func TestLearningObjectiveVerbIsAdvisory(t *testing.T) {
	page := strings.Replace(learningGoodPage, "name the four metrics", "grok the four metrics", 1)
	issues := lintLearningBlock("context/learning/x.md", page, ctxTypeLesson)
	found := false
	for _, i := range issues {
		if i.Priority == ctxLintSuggestion && strings.Contains(i.Message, "grok") {
			found = true
		}
		if i.Priority == ctxLintWarning {
			t.Errorf("an unusual verb produced a WARNING: %s", i.Message)
		}
	}
	if !found {
		t.Error("an objective that opens with an unlisted verb produced no SUGGESTION")
	}
}
