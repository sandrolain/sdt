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
		Body:        "## Claims\n\n1. {#claim-1} <!-- c-aaaaaaaa --> a\n2. {#claim-2} <!-- c-bbbbbbbb --> b\n",
	}
}

// writeLesson writes a concept's lesson into a temp corpus and returns its path.
// A body that still carries an `## Assess` table is split: the table is the
// concept's item bank, so it lands in the assessment companion beside the lesson
// (`instructions/lesson.md`). The split keeps every fixture written before the
// split readable as the shape the reader now expects.
func writeLesson(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	lessonBody, items := splitAssessSection(body)
	path := writeLearningDoc(t, learnLessonPath("architecture/x"), lessonBody)
	if items != "" {
		writeAssessmentFor(t, "architecture/x", items)
	}
	return path
}

// writeAssessmentFor writes the item bank of one concept under the temp corpus.
func writeAssessmentFor(t *testing.T, conceptID, items string) string {
	t.Helper()
	fm := "---\nkind: assessment\nid: " + conceptID + ".assessment\nconcept: " + conceptID + "\n" +
		"summary: the item bank of " + conceptID + "\nstatus: draft\n" +
		"created: 2026-10-28T00:00:00Z\nupdated: 2026-10-28T00:00:00Z\n---\n\n"
	return writeLearningDoc(t, learnAssessmentPath(conceptID), fm+"## Items\n\n"+freezeTable(items)+"\n")
}

// writeAssessment writes an item bank for the concept the bundle page names,
// without a lesson: the shape a page quizzed before it is taught takes.
func writeAssessment(t *testing.T, items string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	return writeAssessmentFor(t, "architecture/x", items)
}

// freezeTable appends the `key_hash` column to an item table that lacks it and
// fills each row with the digest of the four cells that decide a verdict. The
// freeze date is left alone: a fixture that carries none exercises the fallback to
// the document's `created`, and a fixture that carries one keeps its own date.
func freezeTable(table string) string {
	rows := markdownTableRows(table)
	if len(rows) < 2 {
		return table
	}
	header := rows[0]
	for _, cell := range header {
		if strings.EqualFold(strings.TrimSpace(cell), "key_hash") {
			return table
		}
	}
	col := func(name string) int {
		for i, cell := range header {
			if strings.EqualFold(strings.TrimSpace(cell), name) {
				return i
			}
		}
		return -1
	}
	cell := func(cells []string, i int) string {
		if i < 0 || i >= len(cells) {
			return ""
		}
		return strings.TrimSpace(cells[i])
	}
	gradingCol := col("grading")
	expectedCol, rubricCol, distractorCol := col("expected"), col("rubric"), col("distractor")
	var out strings.Builder
	out.WriteString("| " + strings.Join(header, " | ") + " | key_hash |\n")
	out.WriteString("|" + strings.Repeat(" --- |", len(header)+1) + "\n")
	for _, cells := range rows[1:] {
		grading := cell(cells, gradingCol)
		if grading == "" {
			grading = learnGradingAuto
		}
		digest := learn.KeyDigest(cell(cells, expectedCol), cell(cells, rubricCol), grading, cell(cells, distractorCol))
		out.WriteString("| " + strings.Join(cells, " | ") + " | " + digest + " |\n")
	}
	return out.String()
}

// writeLearningDoc writes one document under the temp corpus, creating its
// directories, and returns the path.
func writeLearningDoc(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// splitAssessSection removes the `## Assess` section from a lesson body and
// returns it as the table the assessment document carries. A body without the
// section comes back untouched with no items.
func splitAssessSection(body string) (string, string) {
	start := strings.Index(body, "## Assess")
	if start < 0 {
		return body, ""
	}
	rest := body[start:]
	end := len(rest)
	if next := strings.Index(rest[len("## Assess"):], "\n## "); next >= 0 {
		end = len("## Assess") + next + 1
	}
	section := strings.TrimSpace(rest[:end])
	table := strings.TrimSpace(strings.TrimPrefix(section, "## Assess"))
	return strings.TrimRight(body[:start], "\n") + "\n\n" + strings.TrimLeft(rest[end:], "\n"), table
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
	if issues, _ := learnValidateAll(bld); len(issues) != 0 {
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
		{"unknown claim", strings.Replace(checkLessonBody, "architecture/x#c-bbbbbbbb", "architecture/x#c-99999999", 1), bundlePage(), "does not carry"},
		{"claim of another page", strings.Replace(checkLessonBody, "architecture/x#c-bbbbbbbb", "architecture/y#c-bbbbbbbb", 1), bundlePage(), "want"},
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
			issues, _ := learnValidateAll(bld)
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
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false, false)
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
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false, false)
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
	entry, err := learnQuizEntryFor(&bld, &learn.State{}, "architecture/x", false, false)
	if err != nil {
		t.Fatal(err)
	}
	m := learnQuizManifest{Version: learnQuizVersion, Concepts: []learnQuizEntry{entry}}
	if err := learnQuizValidate(m); err != nil {
		t.Fatalf("generated manifest does not validate: %v", err)
	}
}

// TestLearnBundleCarriesClaimDigests proves the author can read the value the
// `hash` cell needs from the bundle instead of computing it separately.
func TestLearnBundleCarriesClaimDigests(t *testing.T) {
	writeLesson(t, checkLessonBody)
	page := bundlePage()
	bld := &contextwiki.Builder{Pages: []*contextwiki.Page{page}, ByID: map[string]*contextwiki.Page{page.ID: page}}
	b := learnBundleOf(bld, page)
	if len(b.Claims) != 2 {
		t.Fatalf("claims = %d, want 2", len(b.Claims))
	}
	want := contextwiki.Claims(page.Body)
	for i, c := range b.Claims {
		if c.ID != want[i].ID || c.Digest != want[i].Digest {
			t.Errorf("claim %d = %+v, want id %s digest %s", i, c, want[i].ID, want[i].Digest)
		}
		if c.Digest == "" {
			t.Errorf("claim %d has no digest", i)
		}
	}
}

// TestLearnBundleTextPrintsClaimDigests proves the text form carries the value
// without a JSON reader.
func TestLearnBundleTextPrintsClaimDigests(t *testing.T) {
	writeLesson(t, checkLessonBody)
	page := bundlePage()
	bld := &contextwiki.Builder{Pages: []*contextwiki.Page{page}, ByID: map[string]*contextwiki.Page{page.ID: page}}
	text := learnBundleOf(bld, page).text()
	for _, c := range contextwiki.Claims(page.Body) {
		if !strings.Contains(text, "claim "+c.ID+" "+c.Digest) {
			t.Errorf("text does not carry claim %s digest %s:\n%s", c.ID, c.Digest, text)
		}
	}
}

// TestLearnBundleWithoutClaims proves a page with no anchors adds no claims.
func TestLearnBundleWithoutClaims(t *testing.T) {
	writeLesson(t, checkLessonBody)
	page := &contextwiki.Page{
		ID: "architecture/x", Status: ctxWikiStatusActive,
		Frontmatter: "---\n" + checkPageFM + "---\n",
		Body:        "## Claims\n\nNo anchors here.\n",
	}
	bld := &contextwiki.Builder{Pages: []*contextwiki.Page{page}, ByID: map[string]*contextwiki.Page{page.ID: page}}
	b := learnBundleOf(bld, page)
	if len(b.Claims) != 0 {
		t.Errorf("claims = %v, want none", b.Claims)
	}
	if strings.Contains(b.text(), "  claim ") {
		t.Error("the text form printed a claim line for a page with no claims")
	}
}
