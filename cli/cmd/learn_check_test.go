package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// lessonFixture writes a concept page and (optionally) its lesson into a temp
// context/ tree and returns the page as the builder would hand it over.
func lessonFixture(t *testing.T, pageFM, lessonBody string, withLesson bool) *contextwiki.Page {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(sdtLearningDir, "architecture"), 0o750); err != nil {
		t.Fatal(err)
	}
	fm := "---\n" + pageFM + "---\n\n## Summary\n\nx\n"
	if withLesson {
		if err := os.WriteFile(learnLessonPath("architecture/x"), []byte(lessonBody), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &contextwiki.Page{
		ID:          "architecture/x",
		Title:       "X",
		Status:      ctxWikiStatusActive,
		ClaimCount:  ctxLessonMinClaims,
		Frontmatter: fm,
	}
}

const checkPageFM = "kind: wiki\nid: architecture/x\n" +
	"learning:\n  objectives:\n    - \"name the parts\"\n    - \"apply the rule\"\n  difficulty: intermediate\n  minutes: 10\n"

const checkLessonBody = `## Prerequisites

## Worked example

` + "`source_span`" + `: refs/x.md@5772ca09:1-2

## Assess

| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| a-1 | 1 | recall | name them | the parts | all parts | one part | architecture/x#claim-1 |
| a-2 | 2 | transfer | apply it here | the answer | matches | the wrong rule | architecture/x#claim-2 |

## Transfer

A case the page did not cover: apply the rule to a queue nobody described.
`

// TestReadinessTeachesAShapedConcept is the green case: an active page, four
// claims, a learning block and a lesson whose objectives are covered.
func TestReadinessTeachesAShapedConcept(t *testing.T) {
	p := lessonFixture(t, checkPageFM, checkLessonBody, true)
	e := learnReadinessOf(p)
	if e.Verdict != learnVerdictTeachable {
		t.Fatalf("verdict = %q (%v), want teachable", e.Verdict, e.Reasons)
	}
	if e.Items != 2 || e.Objectives != 2 {
		t.Errorf("items=%d objectives=%d, want 2 and 2", e.Items, e.Objectives)
	}
}

// TestReadinessRulesEachRefuse covers the six rules one at a time, so a refusal
// always names what the content is missing.
func TestReadinessRulesEachRefuse(t *testing.T) {
	cases := []struct {
		name    string
		fm      string
		claims  int
		status  string
		lesson  bool
		body    string
		wantSub string
	}{
		{"not active", checkPageFM, 4, ctxWikiStatusDraft, true, checkLessonBody, "page status"},
		{"too few claims", checkPageFM, 3, ctxWikiStatusActive, true, checkLessonBody, "fewer than the 4"},
		{"maturity wip", checkPageFM + "maturity: wip\n", 4, ctxWikiStatusActive, true, checkLessonBody, "maturity is `wip`"},
		{"no learning block", "kind: wiki\nid: architecture/x\n", 4, ctxWikiStatusActive, true, checkLessonBody, "no `learning:` block"},
		{"no lesson", checkPageFM, 4, ctxWikiStatusActive, false, "", "no lesson for this concept"},
		{"objective without item", checkPageFM, 4, ctxWikiStatusActive, true,
			strings.Replace(checkLessonBody, "| a-2 | 2 |", "| a-2 | 1 |", 1), "have no item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := lessonFixture(t, tc.fm, tc.body, tc.lesson)
			p.ClaimCount = tc.claims
			p.Status = tc.status
			e := learnReadinessOf(p)
			if e.Verdict != learnVerdictRefused {
				t.Fatalf("verdict = %q, want refused", e.Verdict)
			}
			joined := strings.Join(e.Reasons, "\n")
			if !strings.Contains(joined, tc.wantSub) {
				t.Errorf("reasons do not mention %q:\n%s", tc.wantSub, joined)
			}
		})
	}
}

// TestStampNeverGates proves the rule the plan carries: a missing verified_at
// changes nothing about teachability, it only changes the note.
func TestStampNeverGates(t *testing.T) {
	p := lessonFixture(t, checkPageFM, checkLessonBody, true)
	e := learnReadinessOf(p)
	if e.Verdict != learnVerdictTeachable {
		t.Fatalf("an unstamped page was refused: %v", e.Reasons)
	}
	if e.Stamp != learnStampUnverified {
		t.Errorf("stamp = %q, want %q", e.Stamp, learnStampUnverified)
	}
	if got := stampedOnly([]learnCheckEntry{e}); len(got) != 0 {
		t.Errorf("--verified-only kept an unstamped concept: %+v", got)
	}

	p.Frontmatter = strings.Replace(p.Frontmatter, "kind: wiki\n", "kind: wiki\nverified_at: 2026-10-28T00:00:00Z\n", 1)
	e = learnReadinessOf(p)
	if e.Stamp != learnStampVerified {
		t.Fatalf("stamp = %q, want %q", e.Stamp, learnStampVerified)
	}
	if got := stampedOnly([]learnCheckEntry{e}); len(got) != 1 {
		t.Errorf("--verified-only dropped a stamped concept")
	}
}

// TestStampedOnlyKeepsTheVerdict proves the filter never launders a refusal.
func TestStampedOnlyKeepsTheVerdict(t *testing.T) {
	in := []learnCheckEntry{
		{Concept: "a/b", Verdict: learnVerdictRefused, Stamp: learnStampVerified},
		{Concept: "c/d", Verdict: learnVerdictTeachable, Stamp: learnStampUnverified},
	}
	got := stampedOnly(in)
	if len(got) != 1 || got[0].Concept != "a/b" || got[0].Verdict != learnVerdictRefused {
		t.Errorf("filter changed a verdict: %+v", got)
	}
}

// TestUncoveredObjectives reads the objective index out of the item rows.
func TestUncoveredObjectives(t *testing.T) {
	if got := learnUncoveredObjectives(3, []string{"1", "3"}); len(got) != 1 || got[0] != 2 {
		t.Errorf("uncovered = %v, want [2]", got)
	}
	if got := learnUncoveredObjectives(0, nil); len(got) != 0 {
		t.Errorf("a page with no objectives reported %v", got)
	}
}
