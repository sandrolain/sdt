package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// bundlePage is a page the builder would hand over: a learning block and two
// claim anchors in the body.
func bundlePage() *contextwiki.Page {
	return &contextwiki.Page{
		ID:          "architecture/x",
		Title:       "X",
		Status:      ctxWikiStatusActive,
		Frontmatter: "---\n" + checkPageFM + "---\n",
		Body:        "## Claims\n\n1. {#claim-1} a\n2. {#claim-2} b\n",
	}
}

// writeLesson writes a lesson document into a temp corpus and returns its path.
func writeLesson(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	path := learnLessonPath("architecture/x")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLearnBundleComposesALesson proves the bundle carries what a tutor needs:
// objectives, pacing, the example with its span, the items and the transfer case.
func TestLearnBundleComposesALesson(t *testing.T) {
	writeLesson(t, checkLessonBody)
	bld := &contextwiki.Builder{Pages: nil, ByID: map[string]*contextwiki.Page{}}
	bld.ByID[bundlePage().ID] = bundlePage()
	bld.Pages = append(bld.Pages, bundlePage())
	b := learnBundleOf(bld, bundlePage())
	if len(b.Objectives) != 2 {
		t.Errorf("objectives = %v", b.Objectives)
	}
	if b.Difficulty != "intermediate" || b.Minutes != 10 {
		t.Errorf("pacing = %q/%d", b.Difficulty, b.Minutes)
	}
	if len(b.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(b.Items))
	}
	if b.Example.SourceSpan == "" {
		t.Error("the worked example lost its source span")
	}
	if b.Transfer == "" {
		t.Error("the transfer case is empty")
	}
}

// TestLearnValidateAcceptsAUsableLesson is the green case.
func TestLearnValidateAcceptsAUsableLesson(t *testing.T) {
	writeLesson(t, checkLessonBody)
	bld := &contextwiki.Builder{Pages: nil, ByID: map[string]*contextwiki.Page{}}
	bld.ByID[bundlePage().ID] = bundlePage()
	bld.Pages = append(bld.Pages, bundlePage())
	if issues := learnValidateAll(bld); len(issues) != 0 {
		t.Fatalf("a usable lesson produced %d finding(s): %v", len(issues), issues)
	}
}

// TestLearnValidateRejectsEachFailureMode covers what shape lint cannot see: the
// references that only exist relative to the page.
func TestLearnValidateRejectsEachFailureMode(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		page    *contextwiki.Page
		wantSub string
	}{
		{"no page", checkLessonBody, nil, "no wiki page"},
		{"unknown claim", strings.Replace(checkLessonBody, "architecture/x#claim-2", "architecture/x#claim-9", 1), bundlePage(), "does not carry"},
		{"claim of another page", strings.Replace(checkLessonBody, "architecture/x#claim-2", "architecture/y#claim-2", 1), bundlePage(), "want"},
		{"objective out of range", strings.Replace(checkLessonBody, "| a-2 | 2 |", "| a-2 | 7 |", 1), bundlePage(), "the page has 2"},
		{"objective without item", strings.Replace(checkLessonBody, "| a-2 | 2 |", "| a-2 | 1 |", 1), bundlePage(), "objective 2 has no item"},
		{"duplicate id", strings.Replace(checkLessonBody, "| a-2 | 2 |", "| a-1 | 2 |", 1), bundlePage(), "appears twice"},
		{"no page objectives", checkLessonBody, &contextwiki.Page{
			ID: "architecture/x", Status: ctxWikiStatusActive,
			Frontmatter: "---\nkind: wiki\nid: architecture/x\n---\n",
			Body:        "1. {#claim-1} a\n2. {#claim-2} b\n",
		}, "no `learning.objectives`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeLesson(t, tc.body)
			bld := &contextwiki.Builder{Pages: nil, ByID: map[string]*contextwiki.Page{}}
			if tc.page != nil {
				bld.ByID[tc.page.ID] = tc.page
				bld.Pages = append(bld.Pages, tc.page)
			}
			issues := learnValidateAll(bld)
			joined := strings.Join(issues, "\n")
			if !strings.Contains(joined, tc.wantSub) {
				t.Errorf("findings do not mention %q:\n%s", tc.wantSub, joined)
			}
		})
	}
}

// TestLearnCoverageReportsUncoveredObjectives proves the matrix is computed from
// the lesson's own rows.
func TestLearnCoverageReportsUncoveredObjectives(t *testing.T) {
	writeLesson(t, strings.Replace(checkLessonBody, "| a-2 | 2 |", "| a-2 | 1 |", 1))
	bld := &contextwiki.Builder{Pages: nil, ByID: map[string]*contextwiki.Page{}}
	bld.ByID[bundlePage().ID] = bundlePage()
	bld.Pages = append(bld.Pages, bundlePage())
	rows := learnCoverage(bld)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if len(rows[0].Uncovered) != 1 || rows[0].Uncovered[0] != 2 {
		t.Errorf("uncovered = %v, want [2]", rows[0].Uncovered)
	}
	if !strings.Contains(rows[0].line(), "uncovered") {
		t.Errorf("coverage line = %q", rows[0].line())
	}
}

// TestQuizCarriesTheLessonItems proves the manifest asks the questions the lesson
// already holds — and never ships their answers, which is the zero-hint promise.
func TestQuizCarriesTheLessonItems(t *testing.T) {
	writeLesson(t, checkLessonBody)
	bld := contextwiki.Builder{ByID: map[string]*contextwiki.Page{}}
	bld.ByID["architecture/x"] = bundlePage()
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false)
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	if len(entry.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(entry.Items))
	}
	if entry.Items[0].Prompt == "" || entry.Items[0].ID == "" || entry.Items[0].Type == "" {
		t.Errorf("item lost a field: %+v", entry.Items[0])
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"expected"`) || strings.Contains(string(raw), `"rubric"`) {
		t.Errorf("the manifest ships an answer: %s", raw)
	}
	if entry.Note != "" {
		t.Errorf("a concept with a lesson reported %q", entry.Note)
	}
}

// TestQuizWithoutALessonSaysSo keeps the old contract visible: no lesson, no
// questions, and the manifest says whose job they are.
func TestQuizWithoutALessonSaysSo(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	bld := contextwiki.Builder{ByID: map[string]*contextwiki.Page{}}
	bld.ByID["architecture/x"] = bundlePage()
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false)
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	if len(entry.Items) != 0 {
		t.Errorf("items = %d, want none", len(entry.Items))
	}
	if !strings.Contains(entry.Note, "yours to write") {
		t.Errorf("note = %q", entry.Note)
	}
}

// TestQuizValidateRejectsBadItems covers the item half of the manifest check.
func TestQuizValidateRejectsBadItems(t *testing.T) {
	base := func(it learnQuizItem) learnQuizManifest {
		return learnQuizManifest{Version: learnQuizVersion, Concepts: []learnQuizEntry{{
			Concept: "architecture/x", Wiki: "architecture/x", Items: []learnQuizItem{it},
		}}}
	}
	cases := []struct {
		name string
		m    learnQuizManifest
		want string
	}{
		{"empty prompt", base(learnQuizItem{ID: "a", Type: "recall"}), "no prompt"},
		{"unknown type", base(learnQuizItem{ID: "a", Type: "recite", Prompt: "p"}), "not one of"},
		{"duplicate id", base(learnQuizItem{ID: "a", Type: "recall", Prompt: "p"}), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "duplicate id" {
				tc.m.Concepts[0].Items = append(tc.m.Concepts[0].Items, tc.m.Concepts[0].Items[0])
				tc.want = "twice"
			}
			err := learnQuizValidate(tc.m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

// TestQuizManifestRoundTrips proves a generated manifest validates as written.
func TestQuizManifestRoundTrips(t *testing.T) {
	writeLesson(t, checkLessonBody)
	bld := contextwiki.Builder{ByID: map[string]*contextwiki.Page{}}
	bld.ByID["architecture/x"] = bundlePage()
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false)
	if err != nil {
		t.Fatal(err)
	}
	m := learnQuizManifest{Version: learnQuizVersion, Concepts: []learnQuizEntry{entry}}
	if err := learnQuizValidate(m); err != nil {
		t.Fatalf("generated manifest does not validate: %v", err)
	}
}
