package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// claimGateCell is the claim v-1 pins in the gate's fixture: the live claim of
// the v2 page, so a case varies one input at a time.
const claimGateCell = "architecture/v2#c-11111111"

// claimDigest returns the digest of one claim of a page the temp corpus carries.
func claimDigest(t *testing.T, concept string, index int) string {
	t.Helper()
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		t.Fatalf("load wiki: %v", err)
	}
	page := bld.Resolve(concept)
	if page == nil {
		t.Fatalf("page %q not found", concept)
	}
	claims := page.Claims()
	if index >= len(claims) {
		t.Fatalf("page %q carries %d claims, want at least %d", concept, len(claims), index+1)
	}
	return claims[index].Digest
}

// claimGateFixture writes the v2 page and a two-item bank whose first row pins
// `cell` and whatever `hash` builds from the live digest of that claim. The
// second row is always fresh, so a refusal can be told from a blanket one.
func claimGateFixture(t *testing.T, cell string, hash func(live string) string) {
	t.Helper()
	runInTempDir(t)
	writeLearnPage(t, "architecture/v2.md", learnV2Page)
	live, sibling := claimDigest(t, "architecture/v2", 0), claimDigest(t, "architecture/v2", 1)
	bank := "| id | objective | type | prompt | expected | rubric | distractor | claim | hash |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| v-1 | 1 | recall | name it | alpha; beta | both | gamma | " + cell + " | " + hash(live) + " |\n" +
		"| v-2 | 2 | apply | apply it | 2/4 = 50% | the ratio | 2 per day | architecture/v2#c-22222222 | " + sibling + " |\n"
	writeAssessmentFor(t, "architecture/v2", bank)
}

// TestClaimDigestGatesEveryReadPath is the delivery matrix: three read paths by
// four claim outcomes, each cell asserted on its own so no cell passes behind
// another. `ok` and `unverified` are served, `stale` and `broken` are refused.
func TestClaimDigestGatesEveryReadPath(t *testing.T) {
	cases := []struct {
		outcome string
		cell    string
		hash    func(live string) string
		served  bool
	}{
		{outcome: learnClaimOK, cell: claimGateCell, hash: func(live string) string { return live }, served: true},
		{outcome: learnClaimUnverified, cell: claimGateCell, hash: func(string) string { return "" }, served: true},
		{outcome: learnClaimStale, cell: claimGateCell, hash: func(string) string { return "0000000000000000" }, served: false},
		{outcome: learnClaimBroken, cell: "architecture/v2#c-99999999", hash: func(live string) string { return live }, served: false},
	}
	for _, tc := range cases {
		t.Run(tc.outcome+"/quiz", func(t *testing.T) {
			claimGateFixture(t, tc.cell, tc.hash)
			if !tc.served {
				shouldExitWithCode(t, 1, func() string {
					return string(execute(t, learnQuizCmd, nil, "architecture/v2", "--format", "json"))
				})
				return
			}
			var manifest learnQuizManifest
			if err := json.Unmarshal(execute(t, learnQuizCmd, nil, "architecture/v2", "--format", "json"), &manifest); err != nil {
				t.Fatalf("quiz manifest: %v", err)
			}
			if len(manifest.Concepts) != 1 || len(manifest.Concepts[0].Items) != 2 {
				t.Errorf("a served bank lost its items: %+v", manifest.Concepts)
			}
		})
		t.Run(tc.outcome+"/answer", func(t *testing.T) {
			claimGateFixture(t, tc.cell, tc.hash)
			if !tc.served {
				shouldExitWithCode(t, 4, func() string {
					return string(execute(t, learnAnswerCmd, nil,
						"--concept", "architecture/v2", "--item", "v-1", "--answer", "alpha"))
				})
				return
			}
			out := execute(t, learnAnswerCmd, nil,
				"--concept", "architecture/v2", "--item", "v-1", "--answer", "alpha")
			if !strings.Contains(string(out), learn.GradeCorrect) {
				t.Errorf("a served item was not graded: %s", out)
			}
		})
		t.Run(tc.outcome+"/record", func(t *testing.T) {
			claimGateFixture(t, tc.cell, tc.hash)
			if !tc.served {
				shouldExitWithCode(t, 4, func() string {
					return string(execute(t, learnRecordCmd, nil,
						"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0"))
				})
				if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
					t.Errorf("a refused record wrote learner state: %v", err)
				}
				return
			}
			execute(t, learnRecordCmd, nil,
				"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0")
			if _, err := os.Stat(learn.Path(".")); err != nil {
				t.Errorf("a served record wrote no state: %v", err)
			}
		})
	}
}

// TestLearnUntouchedBankStaysServed proves the gate reads a bank with no `hash`
// column as unverified: the corpus items the predicate must not refuse are the
// ones nobody re-froze.
func TestLearnUntouchedBankStaysServed(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)

	var manifest learnQuizManifest
	if err := json.Unmarshal(execute(t, learnQuizCmd, nil, "architecture/v2", "--format", "json"), &manifest); err != nil {
		t.Fatalf("quiz manifest: %v", err)
	}
	if len(manifest.Concepts) != 1 || len(manifest.Concepts[0].Items) != 4 {
		t.Errorf("an untouched bank lost items: %+v", manifest.Concepts)
	}
	out := execute(t, learnAnswerCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--answer", "alpha")
	if !strings.Contains(string(out), learn.GradeCorrect) {
		t.Errorf("an untouched item was not graded: %s", out)
	}
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0")
	if _, err := os.Stat(learn.Path(".")); err != nil {
		t.Errorf("an untouched bank could not be recorded: %v", err)
	}
}

// TestLearnClaimRefusalNamesTheCells proves a refusal carries what a reader needs
// to re-freeze the row: the item, the cell it pins, and the page's live digest.
func TestLearnClaimRefusalNamesTheCells(t *testing.T) {
	item := learnLessonItem{ID: "v-1", Claim: claimGateCell, Hash: "aaaa"}
	stale := learnClaimRefused("architecture/v2", item, learnClaimStale, "bbbb", "grading").Error()
	for _, want := range []string{"v-1", "c-11111111", "aaaa", "bbbb", "stale"} {
		if !strings.Contains(stale, want) {
			t.Errorf("the stale refusal does not name %q: %s", want, stale)
		}
	}
	broken := learnClaimRefused("architecture/v2", item, learnClaimBroken, "", "grading").Error()
	for _, want := range []string{"v-1", "does not carry", "broken"} {
		if !strings.Contains(broken, want) {
			t.Errorf("the broken refusal does not name %q: %s", want, broken)
		}
	}
}
