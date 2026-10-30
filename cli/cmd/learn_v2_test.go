package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

const learnV2Page = `---
kind: wiki
id: architecture/v2
title: V2 concept
type: concept
status: active
summary: a concept with a graded lesson
learning:
  objectives:
    - "name it"
    - "apply it"
---
## Summary

V2.

## Claims

1. {#claim-1} it is named (refs/v2.md@abcd1234:1-2)
2. {#claim-2} it is applied (refs/v2.md@abcd1234:3-4)

## Notes

Notes.
`

const learnV2LessonFM = `---
kind: lesson
id: architecture/v2
concept: architecture/v2
title: V2 — lesson
summary: "graded lesson"
status: draft
created: 2026-10-01T00:00:00Z
updated: 2026-10-02T00:00:00Z
project: p
links:
  - wiki/architecture/v2.md

sources:
  - wiki/architecture/v2.md
---

## Prerequisites

## Worked example

` + "`source_span`" + `: refs/v2.md@abcd1234:1-2

`

const learnV2LessonOptional = learnV2LessonFM + `## Assess

| id | objective | type | prompt | expected | rubric | distractor | claim | grading | provenance | frozen |
|---|---|---|---|---|---|---|---|---|---|---|
| v-1 | 1 | recall | name it | alpha; beta | both | gamma | architecture/v2#claim-1 | exact | material | 2026-10-01 |
| v-2 | 2 | apply | apply it | 2/4 = 50% | the ratio | 2 per day | architecture/v2#claim-2 | numeric | material | 2026-10-01 |
| v-3 | 2 | transfer | apply it here | names the window and the zero days | names both | the raw count | architecture/v2#claim-2 | keywords | ai_generated | 2026-10-01 |
| v-4 | 2 | transfer | explain it | any defensible answer | judged | a guess | architecture/v2#claim-2 | manual | unknown | 2026-10-01 |

## Transfer

Another case.
`

const learnV2LessonBase = learnV2LessonFM + `## Assess

| id | objective | type | prompt | expected | rubric | distractor | claim |
|---|---|---|---|---|---|---|---|
| v-1 | 1 | recall | name it | alpha; beta | both | gamma | architecture/v2#claim-1 |
| v-2 | 2 | apply | apply it | 2/4 = 50% | the ratio | 2 per day | architecture/v2#claim-2 |
| v-3 | 2 | transfer | apply it here | names the window and the zero days | names both | the raw count | architecture/v2#claim-2 |

## Transfer

Another case.
`

// learnV2Fixture writes the v2 page and its graded lesson into the temp corpus.
func learnV2Fixture(t *testing.T) {
	t.Helper()
	writeLearnPage(t, "architecture/v2.md", learnV2Page)
	learnV2Lesson(t, learnV2LessonOptional)
}

// learnV2Lesson writes a lesson body for the v2 page.
func learnV2Lesson(t *testing.T, body string) {
	t.Helper()
	path := learnLessonPath("architecture/v2")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// loadV2Bundle composes the v2 lesson from the temp corpus.
func loadV2Bundle(t *testing.T) learnLessonBundle {
	t.Helper()
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		t.Fatalf("load wiki: %v", err)
	}
	page := bld.Resolve("architecture/v2")
	if page == nil {
		t.Fatal("v2 page not found")
	}
	return learnBundleOf(bld, page)
}

// TestLearnLessonReadsOptionalColumns proves the three optional columns are read
// by name and that a lesson without them takes the documented defaults.
func TestLearnLessonReadsOptionalColumns(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	b := loadV2Bundle(t)
	if len(b.Items) != 4 {
		t.Fatalf("items = %d, want 4", len(b.Items))
	}
	first := b.Items[0]
	if first.Grading != "exact" || first.Provenance != "material" || first.Frozen != "2026-10-01" {
		t.Errorf("optional columns lost: %+v", first)
	}
	if b.Items[3].Grading != "manual" || b.Items[3].Provenance != "unknown" {
		t.Errorf("the manual item lost its rule: %+v", b.Items[3])
	}

	// The base table still parses, with every optional value at its default.
	learnV2Lesson(t, learnV2LessonBase)
	b = loadV2Bundle(t)
	if len(b.Items) != 3 {
		t.Fatalf("base items = %d, want 3", len(b.Items))
	}
	for _, it := range b.Items {
		if it.Grading != learn.RuleAuto || it.Provenance != "unknown" {
			t.Errorf("defaults not applied: %+v", it)
		}
		if it.Frozen != "2026-10-01" {
			t.Errorf("freeze must fall back to the lesson created date, got %q", it.Frozen)
		}
	}
}

// TestLearnLessonLintAcceptsOptionalColumns proves the lint accepts the three
// trailing columns and refuses any other, which the parser would ignore.
func TestLearnLessonLintAcceptsOptionalColumns(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	if issues := lintLesson("lesson.md", learnV2LessonOptional, ctxTypeLesson); len(issues) != 0 {
		t.Fatalf("a lesson with the optional columns produced findings: %+v", issues)
	}
	unknown := strings.Replace(learnV2LessonOptional,
		"| claim | grading | provenance | frozen |",
		"| claim | grading | provenance | notes |", 1)
	issues := lintLesson("lesson.md", unknown, ctxTypeLesson)
	joined := ""
	for _, i := range issues {
		joined += i.Message + "\n"
	}
	if !strings.Contains(joined, "not an optional trailing column") {
		t.Errorf("an unknown column must be refused, got:\n%s", joined)
	}
}

// TestLearnValidateRejectsBadOptionalValues covers the value half of the contract:
// an unknown grading rule, an unknown provenance and a freeze after the update.
func TestLearnValidateRejectsBadOptionalValues(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"grading", strings.Replace(learnV2LessonOptional, "| exact | material |", "| fuzzy | material |", 1), "grading \"fuzzy\""},
		{"provenance", strings.Replace(learnV2LessonOptional, "| exact | material |", "| exact | synthetic |", 1), "provenance \"synthetic\""},
		{"frozen after update", strings.Replace(learnV2LessonOptional, "| material | 2026-10-01 |", "| material | 2026-10-09 |", 1), "after the lesson was last updated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runInTempDir(t)
			learnV2Fixture(t)
			learnV2Lesson(t, tc.body)
			bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(learnValidateAll(bld), "\n")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("findings do not mention %q:\n%s", tc.want, joined)
			}
		})
	}
}

// TestLearnAnswerExitCodes proves the grader's contract: 0 for a correct answer, 1
// for an incorrect one, 4 for an ungradable item or one not in the corpus.
func TestLearnAnswerExitCodes(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)

	var report learnAnswerReport
	out := execute(t, learnAnswerCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--answer", "Beta.", "--format", "json")
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("answer json: %v", err)
	}
	if report.Grade != learn.GradeCorrect || report.Rule != learn.RuleExact {
		t.Errorf("verdict = %+v", report)
	}
	if report.Expected != "alpha; beta" || report.Rubric != "both" {
		t.Errorf("the verdict must carry the frozen key and rubric: %+v", report)
	}

	// A correct answer leaves the command normally, which is exit 0: the grader
	// only calls exit for a non-zero verdict.
	out = execute(t, learnAnswerCmd, nil,
		"--concept", "architecture/v2", "--item", "v-2", "--answer", "50%")
	if !strings.Contains(string(out), learn.GradeCorrect) {
		t.Errorf("a correct answer must report %q, got:\n%s", learn.GradeCorrect, out)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnAnswerCmd, nil,
			"--concept", "architecture/v2", "--item", "v-1", "--answer", "gamma"))
	})
	shouldExitWithCode(t, 4, func() string {
		return string(execute(t, learnAnswerCmd, nil,
			"--concept", "architecture/v2", "--item", "v-4", "--answer", "a defensible answer"))
	})
	shouldExitWithCode(t, 4, func() string {
		return string(execute(t, learnAnswerCmd, nil,
			"--concept", "architecture/v2", "--item", "v-99", "--answer", "x"))
	})
}

// TestLearnAnswerIsReadOnly proves grading writes no learner state.
func TestLearnAnswerIsReadOnly(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	execute(t, learnAnswerCmd, nil, "--concept", "architecture/v2", "--item", "v-1", "--answer", "alpha")
	if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
		t.Errorf("grading wrote learner state: %v", err)
	}
}

// TestLearnQuizKeyIsOptIn proves the default manifest stays key-free and that
// --with-key is the only way the frozen key enters it.
func TestLearnQuizKeyIsOptIn(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)

	var plain learnQuizManifest
	if err := json.Unmarshal(execute(t, learnQuizCmd, nil, "architecture/v2", "--format", "json"), &plain); err != nil {
		t.Fatalf("quiz json: %v", err)
	}
	if plain.Keyed || len(plain.Concepts) != 1 {
		t.Fatalf("default manifest = %+v", plain)
	}
	for _, it := range plain.Concepts[0].Items {
		if it.Expected != "" || it.Rubric != "" {
			t.Errorf("the default manifest ships a key: %+v", it)
		}
	}

	var keyed learnQuizManifest
	if err := json.Unmarshal(execute(t, learnQuizCmd, nil,
		"architecture/v2", "--with-key", "--format", "json"), &keyed); err != nil {
		t.Fatalf("keyed quiz json: %v", err)
	}
	if !keyed.Keyed || keyed.Concepts[0].Items[0].Expected != "alpha; beta" {
		t.Errorf("--with-key must carry the key: %+v", keyed)
	}
	if got := keyed.Concepts[0].Items[0].Frozen; got != "2026-10-01" {
		t.Errorf("the keyed item must carry its freeze date, got %q", got)
	}
	if err := learnQuizValidate(keyed); err != nil {
		t.Errorf("a keyed manifest must validate: %v", err)
	}
	// A key smuggled into a manifest that claims no key is refused.
	plain.Concepts[0].Items[0].Expected = "alpha; beta"
	if err := learnQuizValidate(plain); err == nil {
		t.Error("a key in a manifest that claims no key must be refused")
	}
}

// TestLearnRecordGradeTransition covers the state machine: the ceiling, the
// reason-coded date, the stored attempt and the explicit-date override.
func TestLearnRecordGradeTransition(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)

	// A first correct answer never reaches solid, and schedules independently.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0")
	c := loadRecorded(t).Concepts[0]
	if c.LastAssessed.Understanding != learn.UnderstandingPartial {
		t.Errorf("a first answer must be capped at partial, got %q", c.LastAssessed.Understanding)
	}
	if c.NextReview != "2026-10-20" || c.ScheduleReason != learn.ReasonIndependentSuccess {
		t.Errorf("schedule = %q/%q", c.NextReview, c.ScheduleReason)
	}
	if c.LastAttempt == nil || c.LastAttempt.Grade != learn.GradeCorrect ||
		c.LastAttempt.Item != "v-1" || c.LastAttempt.Hint != 0 {
		t.Errorf("the attempt was not stored: %+v", c.LastAttempt)
	}

	// A second independent success reaches solid and earns the durable interval.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0")
	c = loadRecorded(t).Concepts[0]
	if c.LastAssessed.Understanding != learn.UnderstandingSolid {
		t.Errorf("a second demonstration may reach solid, got %q", c.LastAssessed.Understanding)
	}
	if c.NextReview != "2026-11-12" || c.ScheduleReason != learn.ReasonDurableSuccess {
		t.Errorf("durable schedule = %q/%q", c.NextReview, c.ScheduleReason)
	}

	// A correct answer that needed a hint is capped at partial.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "2")
	c = loadRecorded(t).Concepts[0]
	if c.LastAssessed.Understanding != learn.UnderstandingPartial || c.HintLevel != 2 {
		t.Errorf("the hint ceiling did not apply: %+v", c.LastAssessed)
	}
	if c.NextReview != "2026-10-16" || c.ScheduleReason != learn.ReasonCorrectButHesitant {
		t.Errorf("hesitant schedule = %q/%q", c.NextReview, c.ScheduleReason)
	}

	// An incorrect answer fails and comes back tomorrow.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "incorrect")
	c = loadRecorded(t).Concepts[0]
	if c.LastAssessed.Understanding != learn.UnderstandingWeak {
		t.Errorf("an incorrect answer must be weak, got %q", c.LastAssessed.Understanding)
	}
	if c.NextReview != "2026-10-14" || c.ScheduleReason != learn.ReasonFailure {
		t.Errorf("failure schedule = %q/%q", c.NextReview, c.ScheduleReason)
	}

	// A transfer item earns the transfer interval — downgraded one rung because the
	// answer before it was a failure: re-earning takes longer than keeping.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-3", "--grade", "correct", "--hint", "0")
	c = loadRecorded(t).Concepts[0]
	if c.NextReview != "2026-10-20" || c.ScheduleReason != learn.ReasonIndependentSuccess {
		t.Errorf("transfer schedule after a failure = %q/%q", c.NextReview, c.ScheduleReason)
	}

	// Without a failure behind it, the same transfer earns its full interval.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0")
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-3", "--grade", "correct", "--hint", "0")
	c = loadRecorded(t).Concepts[0]
	if c.NextReview != "2026-10-27" || c.ScheduleReason != learn.ReasonTransferSuccess {
		t.Errorf("transfer schedule = %q/%q", c.NextReview, c.ScheduleReason)
	}

	// An explicit date wins and carries no computed reason.
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "correct", "--hint", "0",
		"--next-review", "2026-12-01")
	c = loadRecorded(t).Concepts[0]
	if c.NextReview != "2026-12-01" || c.ScheduleReason != "" {
		t.Errorf("explicit date = %q/%q", c.NextReview, c.ScheduleReason)
	}
}

// TestLearnRecordGradeRefusals proves the guarded inputs: an ungradable answer
// writes nothing, and the conflicting or incomplete calls are refused.
func TestLearnRecordGradeRefusals(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	shouldExitWithCode(t, 4, func() string {
		return string(execute(t, learnRecordCmd, nil,
			"--concept", "architecture/v2", "--item", "v-4", "--grade", "ungradable"))
	})
	if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
		t.Fatalf("an ungradable answer must write nothing, got %v", err)
	}

	for _, args := range [][]string{
		{"--concept", "architecture/v2", "--grade", "correct", "--understanding", "partial", "--item", "v-1"},
		{"--concept", "architecture/v2", "--item", "v-1"},
		{"--concept", "architecture/v2", "--grade", "correct"},
		{"--concept", "architecture/v2", "--grade", "mastered", "--item", "v-1"},
		{"--concept", "architecture/v2", "--grade", "correct", "--item", "v-99"},
		{"--concept", "architecture/v2", "--grade", "correct", "--item", "v-1", "--hint", "-1"},
	} {
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, learnRecordCmd, nil, args...))
		})
	}
	if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
		t.Errorf("a refused record must leave no state, got %v", err)
	}
}

// TestLearnStatusSeparatesCoverageAndMastery proves the two facts stay two lines
// and are counted from different sets.
func TestLearnStatusSeparatesCoverageAndMastery(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	learnV2Fixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)
	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "partially_correct")
	if err := learn.Save(".", &learn.State{
		Version: learn.Version,
		Concepts: append(loadRecorded(t).Concepts, learn.Concept{
			ID: "backend-auth", Wiki: "backend/auth",
		}),
	}); err != nil {
		t.Fatal(err)
	}

	var report learnStatusReport
	if err := json.Unmarshal(execute(t, learnStatusCmd, nil, "--format", "json"), &report); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if report.Pages != 3 {
		t.Fatalf("pages = %d, want 3", report.Pages)
	}
	if report.Coverage.Assessed != 1 || report.Coverage.Covered != 1 || report.Coverage.Unseen != 1 {
		t.Errorf("coverage = %+v", report.Coverage)
	}
	if report.Mastery.Partial != 1 || report.Mastery.Unknown != 1 || report.Mastery.Solid != 0 {
		t.Errorf("mastery = %+v", report.Mastery)
	}
	out := string(execute(t, learnStatusCmd, nil))
	for _, want := range []string{
		"coverage: 1 assessed, 1 covered, 1 unseen of 3 page(s)",
		"mastery: 0 solid, 1 partial, 0 weak, 1 unknown",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q, got:\n%s", want, out)
		}
	}
}

// TestLearnResumeCapsNextActions proves the ranking: due first, at most two, each
// with a why, and a blocked prerequisite outranks a new page.
func TestLearnResumeCapsNextActions(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	learnV2Fixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)

	st := &learn.State{
		Version: learn.Version,
		Concepts: []learn.Concept{
			{ID: "architecture-v2", Wiki: "architecture/v2",
				LastAssessed: &learn.Assessment{At: "2026-10-01T09:00:00Z",
					Basis: learn.BasisObserved, Understanding: learn.UnderstandingWeak},
				NextReview:     now.AddDate(0, 0, -1).Format(learn.DateLayout),
				ScheduleReason: learn.ReasonFailure},
			{ID: "backend-auth", Wiki: "backend/auth", Prerequisites: []string{"backend-registry"},
				LastAssessed: &learn.Assessment{At: "2026-10-01T09:00:00Z",
					Basis: learn.BasisObserved, Understanding: learn.UnderstandingPartial},
				NextReview: now.AddDate(0, 0, 5).Format(learn.DateLayout)},
		},
	}
	if err := learn.Save(".", st); err != nil {
		t.Fatal(err)
	}

	var report learnResumeReport
	if err := json.Unmarshal(execute(t, learnResumeCmd, nil, "--format", "json"), &report); err != nil {
		t.Fatalf("resume json: %v", err)
	}
	if len(report.Actions) != learnMaxActions {
		t.Fatalf("actions = %+v, want %d", report.Actions, learnMaxActions)
	}
	if report.Actions[0].Concept != "architecture-v2" || report.Actions[0].Action != "review" {
		t.Errorf("the due concept must come first, got %+v", report.Actions[0])
	}
	if !strings.HasPrefix(report.Actions[0].Why, "due ") {
		t.Errorf("the action must say why, got %+v", report.Actions[0])
	}
	if report.Actions[1].Concept != "backend-auth" || report.Actions[1].Action != "revisit" {
		t.Errorf("the weak concept must come second, got %+v", report.Actions[1])
	}

	out := string(execute(t, learnResumeCmd, nil))
	_, actions, found := strings.Cut(out, "next:\n")
	if !found {
		t.Fatalf("resume must print a next block, got:\n%s", out)
	}
	if got := strings.Count("\n"+actions, "\n  "); got != learnMaxActions {
		t.Errorf("resume must print exactly %d action(s), got %d:\n%s", learnMaxActions, got, out)
	}
}

// TestLearnReviewShowsScheduleReason proves the queue explains its date.
func TestLearnReviewShowsScheduleReason(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)
	if err := learn.Save(".", &learn.State{
		Version: learn.Version,
		Concepts: []learn.Concept{{
			ID: "backend-auth", Wiki: "backend/auth",
			LastAssessed: &learn.Assessment{At: "2026-10-01T09:00:00Z",
				Basis: learn.BasisObserved, Understanding: learn.UnderstandingWeak},
			NextReview:     now.AddDate(0, 0, -1).Format(learn.DateLayout),
			ScheduleReason: learn.ReasonFailure,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, learnReviewCmd, nil))
	if !strings.Contains(out, learn.ReasonFailure) || !strings.Contains(out, "-1d") {
		t.Errorf("review must show the reason and the distance, got:\n%s", out)
	}
}

// TestLearnEvalRunsCases proves the harness: a passing case exits 0, a failing one
// exits 1, and the schedule half is checked when a case asks for it.
func TestLearnEvalRunsCases(t *testing.T) {
	runInTempDir(t)
	dir := filepath.Join(sdtLearningDir, "evals")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	passing := `- name: exact-ok
  rule: exact
  item_type: recall
  expected: "alpha; beta"
  answer: "Beta."
  grade: correct
- name: schedule-ok
  rule: exact
  item_type: recall
  expected: "alpha"
  answer: "lead"
  grade: incorrect
  prior_reason: durable_success
  want_reason: failure
  want_days: 1
`
	if err := os.WriteFile(filepath.Join(dir, "cases.yaml"), []byte(passing), 0o600); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, learnEvalCmd, nil))
	if !strings.Contains(out, "2 case(s), 0 failed") {
		t.Errorf("a passing set must report zero failures, got:\n%s", out)
	}

	failing := strings.Replace(passing, "grade: correct", "grade: incorrect", 1)
	if err := os.WriteFile(filepath.Join(dir, "cases.yaml"), []byte(failing), 0o600); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnEvalCmd, nil))
	})
}

// TestLearnEvalWithoutCasesIsHonest proves a project with no eval directory reports
// zero cases rather than a false green.
func TestLearnEvalWithoutCasesIsHonest(t *testing.T) {
	runInTempDir(t)
	out := string(execute(t, learnEvalCmd, nil))
	if !strings.Contains(out, "no eval cases") {
		t.Errorf("an empty harness must say so, got:\n%s", out)
	}
}
