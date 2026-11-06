package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ── the item bank of one concept ──────────────────────────────────────────────
//
// An assessment is the item bank of exactly one concept: a corpus document beside
// the lesson, `context/learning/<concept>.assessment.md`, carrying
// `kind: assessment`, `id: <concept>.assessment` and `concept: <concept>`
// (`instructions/lesson.md`). The key lives here and never in the lesson, so the
// document a learner is shown does not carry it.
//
// Two digests, two duties: `hash` pins the wiki claim the item was written
// against (`learn lesson --validate` reports a stale claim), `key_hash` pins the
// cells that decide a verdict (expected, rubric, grading, distractor).

// learnAssessmentSection is the only section an assessment body carries.
const learnAssessmentSection = "Items"

// errLearnBankNotCurrent names the refusal of an item bank whose stored digest no
// longer agrees with its row. One condition, one exit code: every read path
// answers it with 4, so the distinction a message cannot carry stays in the
// error rather than in its text.
var errLearnBankNotCurrent = errors.New("the item bank is not current")

// learnBankNotCurrent carries the specific finding while unwrapping to the
// condition above: the message stays the one the gate wrote, and a caller takes
// the exit code from the condition instead of guessing it from the sentence.
type learnBankNotCurrent struct{ msg string }

func (e learnBankNotCurrent) Error() string { return e.msg }

func (e learnBankNotCurrent) Unwrap() error { return errLearnBankNotCurrent }

// learnRefuseBank marks a refusal as the bank-not-current condition.
func learnRefuseBank(err error) error {
	return learnBankNotCurrent{msg: err.Error()}
}

// learnAssessmentPath is the item bank of a concept: the companion of its lesson.
func learnAssessmentPath(conceptID string) string {
	return filepath.Join(sdtLearningDir, filepath.FromSlash(conceptID)+ctxAssessmentSuffix+sdtMarkdownExt)
}

// learnIsAssessmentPath reports whether a corpus path is an item bank rather than
// a lesson. The two share a directory, so the suffix is the only discriminator.
func learnIsAssessmentPath(path string) bool {
	return strings.HasSuffix(path, ctxAssessmentSuffix+sdtMarkdownExt)
}

// learnIsLessonPath reports whether a corpus path is a lesson, asked of the
// registry rather than of a list of names. The lesson kind carries no suffix of
// its own, so the suffixes its siblings declare in the same directory decide: a
// theme or a syllabus is skipped, and a document no kind claims is still a
// lesson to this walk — an unregistered file must not become invisible.
func learnIsLessonPath(path string) bool {
	t, ok := ctxTypeLookup(ctxTypeLesson)
	return ok && ctxFileMatchesKind(t, path)
}

// learnConceptOfAssessmentPath recovers the concept an assessment path names.
func learnConceptOfAssessmentPath(path string) (string, bool) {
	rel, err := filepath.Rel(sdtLearningDir, path)
	if err != nil {
		return "", false
	}
	slash := filepath.ToSlash(rel)
	if !strings.HasSuffix(slash, ctxAssessmentSuffix+sdtMarkdownExt) {
		return "", false
	}
	return strings.TrimSuffix(slash, ctxAssessmentSuffix+sdtMarkdownExt), true
}

// learnLessonExists reports whether the concept carries a lesson document.
func learnLessonExists(conceptID string) bool {
	info, err := os.Stat(learnLessonPath(conceptID))
	return err == nil && !info.IsDir()
}

// learnAssessmentItems reads the item bank of a concept, with each item's
// effective freeze date. ok is false when the concept carries no assessment —
// not an error: a page can be quizzed before it is taught.
func learnAssessmentItems(conceptID string) ([]learnLessonItem, bool) {
	data, err := os.ReadFile(learnAssessmentPath(conceptID)) //#nosec G304 -- path built from the page id
	if err != nil {
		return nil, false
	}
	return learnAssessmentItemsOf(string(data)), true
}

// learnAssessmentItemsOf reads the rows of an assessment document's `## Items`
// table and stamps each with the freeze date it resolves to.
func learnAssessmentItemsOf(content string) []learnLessonItem {
	items := learnLessonItemsOf(sectionOf(content, learnAssessmentSection))
	created := parseFrontmatterField(content, "created")
	for i := range items {
		items[i].Frozen = learnEffectiveFrozen(items[i].Frozen, created)
	}
	return items
}

// learnAssessmentItemByID is the one item a grader or a recorder addresses: the
// lookup both share, so neither can grade an item the bank does not carry.
func learnAssessmentItemByID(conceptID, itemID string) (learnLessonItem, bool) {
	items, _ := learnAssessmentItems(conceptID)
	for _, it := range items {
		if it.ID == itemID {
			return it, true
		}
	}
	return learnLessonItem{}, false
}

// learnAssessmentStaleItem reports the first item of a bank whose stored key_hash
// disagrees with the row as it now reads. A stored digest that matches, or an
// absent one (an item written before the freeze existed), is not stale: only a
// disagreement means the key moved after it was frozen.
func learnAssessmentStaleItem(conceptID string) (learnLessonItem, bool) {
	items, _ := learnAssessmentItems(conceptID)
	for _, it := range items {
		if it.KeyHash != "" && it.KeyHash != it.KeyDigest {
			return it, true
		}
	}
	return learnLessonItem{}, false
}

// learnItemKeyStale reports whether one item's stored freeze digest disagrees with
// its own row.
func learnItemKeyStale(it learnLessonItem) bool {
	return it.KeyHash != "" && it.KeyHash != it.KeyDigest
}

// learnDocText returns a document's text, or "" when it cannot be read: every
// caller already reports a missing file in its own words, so this one does not
// invent a second message.
func learnDocText(path string) string {
	data, err := os.ReadFile(path) //#nosec G304 -- path from the corpus walk
	if err != nil {
		return ""
	}
	return string(data)
}
