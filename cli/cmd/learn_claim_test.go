package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// claimPageFixture is a page whose claims carry the stable id, with the given
// body.
func claimPageFixture(body string) *contextwiki.Page {
	return &contextwiki.Page{
		ID:          "architecture/x",
		Title:       "X",
		Status:      ctxWikiStatusActive,
		Frontmatter: "---\n" + checkPageFM + "---\n",
		Body:        body,
	}
}

// claimLessonFixture builds a two-item lesson that cites one claim of the page by
// its id and pins the given digest ("" leaves the hash column empty).
func claimLessonFixture(p *contextwiki.Page, index int, hash string) string {
	claims := p.Claims()
	if index >= len(claims) {
		panic("claimLessonFixture: index out of range")
	}
	cell := p.ID + "#" + claims[index].ID
	return "## Prerequisites\n\n## Worked example\n\n" +
		"`source_span`: refs/x.md@5772ca09:1-2\n\n## Assess\n\n" +
		"| id | objective | type | prompt | expected | rubric | distractor | claim | hash |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| a-1 | 1 | recall | name it | the parts | all | one | " + cell + " | " + hash + " |\n" +
		"| a-2 | 2 | transfer | apply it | the answer | matches | the wrong rule | " + cell + " | " + hash + " |\n\n" +
		"## Transfer\n\nA case the page did not cover.\n"
}

// validateWith runs the lesson validator against one page and one lesson body.
func validateWith(t *testing.T, p *contextwiki.Page, lessonBody string) ([]string, learnClaimCounts) {
	t.Helper()
	writeLesson(t, lessonBody)
	bld := &contextwiki.Builder{ByID: map[string]*contextwiki.Page{p.ID: p}}
	bld.Pages = append(bld.Pages, p)
	return learnValidateAll(bld)
}

const claimBodyBase = "## Claims\n\n" +
	"1. {#claim-1} <!-- c-aaaaaaaa --> first fact. (refs/x.md@aaaa1111:3)\n" +
	"2. {#claim-2} <!-- c-bbbbbbbb --> second fact. (refs/x.md@aaaa1111:4)\n"

// TestClaimIDSurvivesRenumbering is regression (a): inserting a claim before the
// cited one moves its number, and the item must keep resolving the same fact.
func TestClaimIDSurvivesRenumbering(t *testing.T) {
	base := claimPageFixture(claimBodyBase)
	lesson := claimLessonFixture(base, 1, base.Claims()[1].Digest)

	inserted := claimPageFixture("## Claims\n\n" +
		"1. {#claim-1} <!-- c-aaaaaaaa --> first fact. (refs/x.md@aaaa1111:3)\n" +
		"2. {#claim-2} <!-- c-cccccccc --> an inserted fact. (refs/x.md@aaaa1111:9)\n" +
		"3. {#claim-3} <!-- c-bbbbbbbb --> second fact. (refs/x.md@aaaa1111:4)\n")

	issues, counts := validateWith(t, inserted, lesson)
	if len(issues) != 0 {
		t.Fatalf("a renumbered claim must stay resolvable, got: %v", issues)
	}
	if counts.OK != 2 {
		t.Errorf("ok = %d, want 2", counts.OK)
	}
}

// TestClaimDigestReportsStaleEdit is regression (b): the text changes under an
// unchanged id, so the item that pinned the old digest is stale.
func TestClaimDigestReportsStaleEdit(t *testing.T) {
	base := claimPageFixture(claimBodyBase)
	lesson := claimLessonFixture(base, 1, base.Claims()[1].Digest)

	edited := claimPageFixture("## Claims\n\n" +
		"1. {#claim-1} <!-- c-aaaaaaaa --> first fact. (refs/x.md@aaaa1111:3)\n" +
		"2. {#claim-2} <!-- c-bbbbbbbb --> second fact, revised. (refs/x.md@aaaa1111:4)\n")

	issues, counts := validateWith(t, edited, lesson)
	if counts.Stale != 2 {
		t.Errorf("stale = %d, want 2", counts.Stale)
	}
	if !strings.Contains(strings.Join(issues, "\n"), "is stale") {
		t.Errorf("a stale item must be reported:\n%v", issues)
	}
}

// TestClaimDigestIgnoresRepinnedCitation is regression (c): the source's sha
// moved, the statement did not.
func TestClaimDigestIgnoresRepinnedCitation(t *testing.T) {
	base := claimPageFixture(claimBodyBase)
	lesson := claimLessonFixture(base, 1, base.Claims()[1].Digest)

	repinned := claimPageFixture("## Claims\n\n" +
		"1. {#claim-1} <!-- c-aaaaaaaa --> first fact. (refs/x.md@ffff9999:3)\n" +
		"2. {#claim-2} <!-- c-bbbbbbbb --> second fact. (refs/x.md@ffff9999:41-42)\n")

	issues, counts := validateWith(t, repinned, lesson)
	if len(issues) != 0 || counts.OK != 2 {
		t.Fatalf("re-pinning a citation must not make an item stale: issues=%v counts=%+v", issues, counts)
	}
}

// TestClaimDigestCoversWrappedClaim is regression (d): the corpus has claims whose
// anchor sits on a citation-only line; the digest must cover the statement above.
func TestClaimDigestCoversWrappedClaim(t *testing.T) {
	wrapped := claimPageFixture("## Claims\n\n" +
		"1. A statement that wraps\n   onto a second line.\n   (refs/x.md@aaaa1111:9)\n" +
		"   {#claim-1} <!-- c-aaaaaaaa -->\n" +
		"2. {#claim-2} <!-- c-bbbbbbbb --> second fact. (refs/x.md@aaaa1111:4)\n")
	lesson := claimLessonFixture(wrapped, 0, wrapped.Claims()[0].Digest)
	if got, want := wrapped.Claims()[0].Text, "A statement that wraps onto a second line."; got != want {
		t.Fatalf("wrapped claim text = %q, want %q", got, want)
	}
	issues, counts := validateWith(t, wrapped, lesson)
	if len(issues) != 0 || counts.OK != 2 {
		t.Fatalf("a wrapped claim must digest as one statement: issues=%v counts=%+v", issues, counts)
	}
}

// TestClaimWithoutHashIsUnverified is regression (e): an item that never pinned a
// digest is `unverified` — a warning, never `broken` — and counted.
func TestClaimWithoutHashIsUnverified(t *testing.T) {
	base := claimPageFixture(claimBodyBase)
	lesson := claimLessonFixture(base, 1, "")
	issues, counts := validateWith(t, base, lesson)
	if counts.Unverified != 2 || counts.OK != 0 {
		t.Errorf("counts = %+v, want unverified=2", counts)
	}
	if !strings.Contains(strings.Join(issues, "\n"), "pins no claim digest") {
		t.Errorf("an unpinned item must be reported as unverified:\n%v", issues)
	}
	if strings.Contains(strings.Join(issues, "\n"), "broken") {
		t.Errorf("an unpinned item is not broken:\n%v", issues)
	}
}

// TestClaimBrokenAndLegacyAreReported covers the two remaining outcomes: an id the
// page does not carry, and the positional anchor a migration has not touched.
func TestClaimBrokenAndLegacyAreReported(t *testing.T) {
	base := claimPageFixture(claimBodyBase)
	broken := claimLessonFixture(base, 1, base.Claims()[1].Digest)
	broken = strings.ReplaceAll(broken, "#"+base.Claims()[1].ID, "#c-99999999")
	issues, counts := validateWith(t, base, broken)
	if counts.Broken != 2 {
		t.Errorf("broken = %d, want 2", counts.Broken)
	}
	if !strings.Contains(strings.Join(issues, "\n"), "does not carry (broken)") {
		t.Errorf("a broken claim reference must say so:\n%v", issues)
	}

	legacy := strings.ReplaceAll(claimLessonFixture(base, 1, ""), "#"+base.Claims()[1].ID, "#claim-2")
	legacy = strings.ReplaceAll(legacy, " | hash |", " |")
	legacy = strings.ReplaceAll(legacy, " |\n| a-", " |\n| a-")
	issues, counts = validateWith(t, base, legacy)
	if counts.Legacy != 2 {
		t.Errorf("legacy = %d, want 2 (%v)", counts.Legacy, issues)
	}
	if !strings.Contains(strings.Join(issues, "\n"), "migrate it to the stable id") {
		t.Errorf("a legacy reference must ask to be migrated:\n%v", issues)
	}
}
