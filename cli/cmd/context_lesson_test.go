package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLessonKindIsRegistered proves the kind is one entry in the shared registry:
// a subpath id under context/learning, the draft/active/archived vocabulary, and
// the corpus surfaces a document of that kind needs.
func TestLessonKindIsRegistered(t *testing.T) {
	typ, ok := ctxTypeLookup(ctxTypeLesson)
	if !ok {
		t.Fatal("lesson is not a registered type")
	}
	if typ.dir != sdtLearningDir {
		t.Errorf("dir = %q, want %q", typ.dir, sdtLearningDir)
	}
	if typ.scheme != ctxSchemeSubpath {
		t.Errorf("scheme = %v, want ctxSchemeSubpath (the id mirrors the concept)", typ.scheme)
	}
	if !typ.newSupported || !typ.pathSupported || !typ.listSupported {
		t.Error("a lesson must be creatable, path-resolvable and listable")
	}
	for _, want := range []string{ctxWikiStatusDraft, ctxWikiStatusActive, statusArchived} {
		if !ctxStatusInVocab(typ, want) {
			t.Errorf("status %q missing from the lesson vocabulary", want)
		}
	}
	if typ.defaultStatus != ctxWikiStatusDraft {
		t.Errorf("default status = %q, want %q", typ.defaultStatus, ctxWikiStatusDraft)
	}
	if typ.retiredStatus != statusArchived {
		t.Errorf("retired status = %q, want %q", typ.retiredStatus, statusArchived)
	}
	if ctxIndexRouteForType(typ) != ctxIndexRouteCorpus {
		t.Error("a lesson must enter the corpus index and lint walk")
	}
}

// TestLessonPathMirrorsConcept checks the subpath scheme: a lesson lives at the
// concept's own id, so the file path is the mapping and no lookup table exists.
func TestLessonPathMirrorsConcept(t *testing.T) {
	path, err := contextPath(ctxTypeLesson, "architecture/four-key-metrics", "", "", "")
	if err != nil {
		t.Fatalf("contextPath: %v", err)
	}
	if want := filepath.Join(sdtLearningDir, "architecture", "four-key-metrics.md"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if _, err := contextPath(ctxTypeLesson, "", "", "", ""); err == nil {
		t.Error("a lesson without a slug must be refused")
	}
}

// TestLessonWalkSkipsLearnerState proves the recursive walk a subpath kind needs
// finds nested lessons and still leaves the learner state alone: it is a YAML
// file, not a document.
func TestLessonWalkSkipsLearnerState(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "architecture")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(nested, "four-key-metrics.md"),
		filepath.Join(root, "learner.yaml"),
	} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := walkMarkdownFiles(nested)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0], "four-key-metrics.md") {
		t.Errorf("nested walk = %v, want the single lesson", got)
	}
}

const lessonGoodBody = `---
kind: lesson
id: architecture/four-key-metrics
---

## Prerequisites

## Worked example

` + "`source_span`" + `: refs/x.md@5772ca09:305-347

## Assess

| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| m-1 | 1 | recall | Name the four metrics. | frequency, lead time, failure rate, restore time | all four | the four are targets | architecture/four-key-metrics#claim-1 |
| m-2 | 2 | transfer | A team reports 36 deploys last month. Is that the metric? | no, frequency is per period | names the period | the raw count is the metric | architecture/four-key-metrics#claim-4 |

## Transfer
`

// TestLessonLintAcceptsAShapedLesson is the green case: four ordered sections and
// an item table whose every row is addressable.
func TestLessonLintAcceptsAShapedLesson(t *testing.T) {
	if issues := lintLesson("context/learning/x.md", lessonGoodBody, ctxTypeLesson); len(issues) != 0 {
		t.Fatalf("a well-shaped lesson produced %d finding(s): %+v", len(issues), issues)
	}
}

// TestLessonLintRejectsShapeDrift covers the mechanical defects the check exists
// for, one per case, so a lesson cannot quietly become a page.
func TestLessonLintRejectsShapeDrift(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{"missing section", func(s string) string {
			return strings.Replace(s, "## Transfer\n", "", 1)
		}, "missing"},
		{"reordered sections", func(s string) string {
			return strings.Replace(s, "## Prerequisites\n\n## Worked example", "## Worked example\n\n## Prerequisites", 1)
		}, "the four sections are ordered"},
		{"extra section", func(s string) string {
			return strings.Replace(s, "## Transfer\n", "## Objectives\n\n## Transfer\n", 1)
		}, "unexpected lesson section"},
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
			issues := lintLesson("context/learning/x.md", tc.mutate(lessonGoodBody), ctxTypeLesson)
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

// TestLessonLintIgnoresOtherKinds keeps the check scoped: a wiki page is not a
// lesson and must not be judged by its shape.
func TestLessonLintIgnoresOtherKinds(t *testing.T) {
	if issues := lintLesson("context/wiki/x.md", lessonGoodBody, ctxTypeWiki); len(issues) != 0 {
		t.Errorf("a wiki page was linted as a lesson: %+v", issues)
	}
}
