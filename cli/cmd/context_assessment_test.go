package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAssessmentKindIsRegistered proves the kind is one entry in the shared
// registry: a subpath id under context/learning that carries the kind suffix, the
// draft/active/archived vocabulary, and the corpus surfaces its documents need.
func TestAssessmentKindIsRegistered(t *testing.T) {
	typ, ok := ctxTypeLookup(ctxTypeAssessment)
	if !ok {
		t.Fatal("assessment is not a registered type")
	}
	if typ.dir != sdtLearningDir {
		t.Errorf("dir = %q, want %q (it sits beside its lesson)", typ.dir, sdtLearningDir)
	}
	if typ.scheme != ctxSchemeSubpath {
		t.Errorf("scheme = %v, want ctxSchemeSubpath (the id mirrors the concept)", typ.scheme)
	}
	if typ.suffix != ctxAssessmentSuffix {
		t.Errorf("suffix = %q, want %q (the id names its concept plus the kind)", typ.suffix, ctxAssessmentSuffix)
	}
	if !typ.newSupported || !typ.pathSupported || !typ.listSupported || !typ.templateAllowed {
		t.Error("an assessment must be creatable, path-resolvable, listable and templated")
	}
	for _, want := range []string{ctxWikiStatusDraft, ctxWikiStatusActive, statusArchived} {
		if !ctxStatusInVocab(typ, want) {
			t.Errorf("status %q missing from the assessment vocabulary", want)
		}
	}
	if typ.retiredStatus != statusArchived {
		t.Errorf("retired status = %q, want %q", typ.retiredStatus, statusArchived)
	}
	if !ctxTypeTakesSubpathSlug(ctxTypeAssessment) {
		t.Error("an assessment slug is a concept subpath")
	}
}

// TestAssessmentIDAndPath pins the pair the registry derives from one slug: the
// file sits beside the lesson and its id repeats the concept plus the suffix, so
// a reader can find the items of a page without a directory scan.
func TestAssessmentIDAndPath(t *testing.T) {
	typ, _ := ctxTypeLookup(ctxTypeAssessment)
	if got, want := ctxTypeID(typ, "architecture/four-key-metrics"), "architecture/four-key-metrics.assessment"; got != want {
		t.Errorf("id = %q, want %q", got, want)
	}
	path, err := contextPath(ctxTypeAssessment, "architecture/four-key-metrics", "", "", "")
	if err != nil {
		t.Fatalf("contextPath: %v", err)
	}
	want := filepath.Join(sdtLearningDir, "architecture", "four-key-metrics.assessment.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	lessonPath, err := contextPath(ctxTypeLesson, "architecture/four-key-metrics", "", "", "")
	if err != nil {
		t.Fatalf("contextPath(lesson): %v", err)
	}
	if filepath.Dir(lessonPath) != filepath.Dir(path) {
		t.Errorf("assessment and lesson are not neighbours: %q vs %q", path, lessonPath)
	}
}

// TestAssessmentScaffold pins the document a new assessment starts as: the
// frontmatter identity the reader resolves on, and the item table whose header is
// the contract between the document and the parser.
func TestAssessmentScaffold(t *testing.T) {
	body := contextDefaultBody(ctxTypeAssessment)
	if !strings.Contains(body, "## Items") {
		t.Error("the assessment scaffold carries no `## Items` section")
	}
	for _, column := range []string{"id", "objective", "type", "prompt", "expected", "rubric", "distractor", ctxFrontmatterClaim, "grading", "provenance", "frozen", "hash", "key_hash"} {
		if !strings.Contains(body, "| "+column+" ") {
			t.Errorf("the item table header names no %q column", column)
		}
	}

	fm := contextFrontmatter(ctxTypeAssessment, "Four Key Metrics — assessment", "summary", "", "p", "", "2026-11-03T00:00:00Z", "", "architecture/four-key-metrics", "", "", "", "", nil, nil, nil)
	for _, want := range []string{
		"kind: assessment\n",
		"id: architecture/four-key-metrics.assessment\n",
		ctxFrontmatterConcept + ": architecture/four-key-metrics\n",
		"status: " + ctxWikiStatusDraft + "\n",
	} {
		if !strings.Contains(fm, want) {
			t.Errorf("frontmatter missing %q:\n%s", want, fm)
		}
	}
}

// TestAssessmentNeedsNoLesson proves the kind is not tied to a lesson: the source
// rule a lesson carries (cite its page) is not applied to an assessment, so a
// concept can be quizzed before it is taught.
func TestAssessmentNeedsNoLesson(t *testing.T) {
	if !ctxTypeTakesSubpathSlug(ctxTypeAssessment) {
		t.Skip("the assessment slug is not a subpath")
	}
	// The lesson rule refuses a --source that omits its page. The assessment kind
	// has no such branch: nothing in contextNewCmd's body construction names it.
	if strings.Contains(contextDefaultBody(ctxTypeAssessment), "Prerequisites") {
		t.Error("an assessment repeats the lesson's sections instead of carrying only its items")
	}
}

const assessmentGoodBody = `---
kind: assessment
id: architecture/four-key-metrics.assessment
concept: architecture/four-key-metrics
---

## Items

| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| m-1 | 1 | recall | Name the four metrics. | frequency, lead time, failure rate, restore time | all four | the four are targets | architecture/four-key-metrics#claim-1 |
| m-2 | 2 | transfer | A team reports 36 deploys last month. Is that the metric? | no, frequency is per period | names the period | the raw count is the metric | architecture/four-key-metrics#claim-4 |
`

// TestAssessmentLintAcceptsAShapedBank is the green case: the one section and an
// item table whose every row is addressable.
func TestAssessmentLintAcceptsAShapedBank(t *testing.T) {
	body := strings.Replace(assessmentGoodBody, itemTableOf(assessmentGoodBody), freezeTable(itemTableOf(assessmentGoodBody)), 1)
	if issues := lintAssessment("context/learning/x.assessment.md", body, ctxTypeAssessment); len(issues) != 0 {
		t.Fatalf("a well-shaped assessment produced %d finding(s): %+v", len(issues), issues)
	}
}

// itemTableOf returns the markdown table a body carries.
func itemTableOf(body string) string {
	start := strings.Index(body, "| id |")
	if start < 0 {
		return ""
	}
	return body[start:]
}

// TestAssessmentLintRejectsEachItemDefect covers the mechanical rules the bank
// exists to enforce, one per case.
func TestAssessmentLintRejectsEachItemDefect(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{"missing section", func(s string) string {
			return strings.Replace(s, "## Items\n", "", 1)
		}, "missing"},
		{"extra section", func(s string) string {
			return s + "\n## Notes\n\nno\n"
		}, "unexpected assessment section"},
		{"bad header", func(s string) string {
			return strings.Replace(s, "| id | objective | type |", "| id | goal | type |", 1)
		}, "item table header"},
		{"unknown type", func(s string) string {
			return strings.Replace(s, "| 1 | recall |", "| 1 | recite |", 1)
		}, "type"},
		{"objective out of range", func(s string) string {
			return strings.Replace(s, "| m-1 | 1 |", "| m-1 | zero |", 1)
		}, "1-based index"},
		{"bad claim ref", func(s string) string {
			return strings.Replace(s, "architecture/four-key-metrics#claim-1", "claim 1", 1)
		}, "#claim-N"},
		{"missing expected", func(s string) string {
			return strings.Replace(s, "| frequency, lead time, failure rate, restore time |", "| |", 1)
		}, "expected answer"},
		{"duplicate id", func(s string) string {
			return strings.Replace(s, "| m-2 | 2 |", "| m-1 | 2 |", 1)
		}, "twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := lintAssessment("context/learning/x.assessment.md", tc.mutate(assessmentGoodBody), ctxTypeAssessment)
			if len(issues) == 0 {
				t.Fatalf("%s produced no finding", tc.name)
			}
			joined := ""
			for _, i := range issues {
				joined += i.Message + "\n"
			}
			if !strings.Contains(joined, tc.wantSub) {
				t.Errorf("findings do not mention %q:\n%s", tc.wantSub, joined)
			}
		})
	}
}

// TestAssessmentLintIgnoresOtherKinds keeps the check scoped: a lesson is not an
// assessment and must not be judged by its shape.
func TestAssessmentLintIgnoresOtherKinds(t *testing.T) {
	if issues := lintAssessment("context/learning/x.md", assessmentGoodBody, ctxTypeLesson); len(issues) != 0 {
		t.Errorf("a lesson was linted as an assessment: %+v", issues)
	}
}
