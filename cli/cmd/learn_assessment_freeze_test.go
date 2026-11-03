package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// tempWiki builds a one-page wiki whose page is the bundle fixture.
func tempWiki() *contextwiki.Builder {
	bld := &contextwiki.Builder{ByID: map[string]*contextwiki.Page{}}
	page := bundlePage()
	bld.ByID[page.ID] = page
	bld.Pages = append(bld.Pages, page)
	return bld
}

// TestAssessmentRefusesAStaleKey proves the freeze is mechanical: a row whose
// key_hash disagrees with its cells is not served, and the refusal names the file
// and the row that moved.
func TestAssessmentRefusesAStaleKey(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeAssessment(t, `| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| a-1 | 1 | recall | name them | the parts | all parts | one part | architecture/x#c-aaaaaaaa |
`)
	// Move the rubric without re-freezing: exactly what the digest exists to catch.
	path := learnAssessmentPath("architecture/x")
	stale := strings.Replace(learnDocText(path), "| all parts |", "| all parts and order |", 1)
	writeLearningDoc(t, path, stale)

	if _, ok := learnAssessmentStaleItem("architecture/x"); !ok {
		t.Fatal("an edited rubric did not stale the item")
	}
	entry, err := learnQuizEntryFor(tempWiki(), &learn.State{}, "architecture/x", false, false)
	if err == nil {
		t.Fatalf("a stale bank was served: %+v", entry)
	}
	if !strings.Contains(err.Error(), "key_hash") || !strings.Contains(err.Error(), "a-1") {
		t.Errorf("refusal does not name the row and its digest: %v", err)
	}
}

// TestAssessmentKeyHashIsTheRowsDigest proves a bank written under the contract is
// frozen: the cell carries the digest of the row it sits in.
func TestAssessmentKeyHashIsTheRowsDigest(t *testing.T) {
	t.Chdir(t.TempDir())
	writeAssessment(t, `| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| a-1 | 1 | recall | name them | the parts | all parts | one part | architecture/x#c-aaaaaaaa |
`)
	items, ok := learnAssessmentItems("architecture/x")
	if !ok || len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].KeyHash == "" || items[0].KeyHash != items[0].KeyDigest {
		t.Fatalf("the fixture is not frozen: cell %q, row %q", items[0].KeyHash, items[0].KeyDigest)
	}
}

// TestReadOnlyVerbsEndWithOneNextStep is the closing-step contract: a text reply
// ends with one next verb, and a JSON reply carries the data and none of the
// advice.
func TestReadOnlyVerbsEndWithOneNextStep(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	writeLesson(t, checkLessonBody)

	text := string(execute(t, learnReviewCmd, nil))
	if strings.Count(text, "📍 next:") != 1 {
		t.Fatalf("review must close with exactly one next step:\n%s", text)
	}
	if !strings.HasPrefix(lastLine(text), "📍 next: sdt learn") {
		t.Errorf("the closing line is not a next verb:\n%s", text)
	}

	raw := string(execute(t, learnReviewCmd, nil, "--format", "json"))
	if strings.Contains(raw, "📍") {
		t.Errorf("JSON output carries advice it should not:\n%s", raw)
	}
}

// lastLine returns the last non-empty line of a text reply.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return ""
}
