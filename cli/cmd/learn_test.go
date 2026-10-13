package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/learn"
)

const learnAuthPage = `---
kind: wiki
id: backend/auth
title: Auth resolution order
type: concept
status: active
summary: how a page id resolves
relations:
  depends_on:
    - "[[backend/registry|Registry]]"
tags:
  - backend
sources:
  - refs/auth.md@abcd1234:1-2
---

## Summary

How a page id resolves.

## Claims

1. Resolution is by relative subpath (refs/auth.md@abcd1234:1-2)

## Notes

Notes.
`

const learnRegistryPage = `---
kind: wiki
id: backend/registry
title: Registry
type: concept
status: active
summary: where pages are registered
tags: []
sources: []
---

## Summary

Where pages are registered.

## Claims

1. The registry holds the page table (refs/registry.md@abcd1234:1-2)

## Notes

Notes.
`

// writeLearnPage writes one wiki page fixture.
func writeLearnPage(t *testing.T, slug, body string) {
	t.Helper()
	path := filepath.Join(sdtWikiDir, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// learnWikiFixture writes a two-page corpus where auth depends on registry.
func learnWikiFixture(t *testing.T) {
	t.Helper()
	writeLearnPage(t, "backend/registry.md", learnRegistryPage)
	writeLearnPage(t, "backend/auth.md", learnAuthPage)
}

// learnStateFixture records learner state: a profile, a goal and one assessed
// concept that is both weak and due.
func learnStateFixture(t *testing.T, now time.Time) *learn.State {
	t.Helper()
	st := &learn.State{
		Version: learn.Version,
		Profile: learn.Profile{Name: "a learner", Since: "2026-10-01"},
		Goals:   []learn.Goal{{ID: "g1", Concept: "backend/auth", Outcome: "explain the resolution order", Created: "2026-10-01"}},
		Concepts: []learn.Concept{{
			ID: "backend-auth", Wiki: "backend/auth",
			Evidence:     []string{"refs/auth.md@abcd1234:1-2"},
			LastAssessed: &learn.Assessment{At: "2026-10-12T10:00:00Z", Basis: learn.BasisObserved, Understanding: learn.UnderstandingPartial},
			NextReview:   now.AddDate(0, 0, -2).Format(learn.DateLayout),
		}},
		Sessions: []learn.Session{{At: "2026-10-12T10:05:00Z", Covered: []string{"backend-auth"}}},
	}
	if err := learn.Save(".", st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestLearnInspectResolvesScope covers scope resolution: an exact title, a
// substring match, the prerequisites to teach first, and an honest miss.
func TestLearnInspectResolvesScope(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	out := string(execute(t, learnInspectCmd, nil, "resolution"))
	if !strings.Contains(out, "backend-auth") || !strings.Contains(out, "Auth resolution order") {
		t.Errorf("inspect must resolve the topic, got:\n%s", out)
	}
	if !strings.Contains(out, "prerequisites: backend/registry") {
		t.Errorf("inspect must resolve the prerequisite, got:\n%s", out)
	}
	if strings.Contains(out, "recorded") && strings.Contains(out, "new") {
		t.Errorf("a concept with no record must stand as new, got:\n%s", out)
	}

	// An unknown topic is reported, never answered from recall.
	out = string(execute(t, learnInspectCmd, nil, "quantum chromodynamics"))
	if !strings.Contains(out, "no wiki page matches") {
		t.Errorf("an unresolved topic must say so, got:\n%s", out)
	}

	// --limit bounds the report.
	out = string(execute(t, learnInspectCmd, nil, "backend", "--limit", "1"))
	if strings.Count(out, "\n") > 2 {
		t.Errorf("--limit must bound the report, got:\n%s", out)
	}
}

// TestLearnInspectRequiresTopic proves the verb refuses an empty subject instead
// of resolving the whole corpus.
func TestLearnInspectRequiresTopic(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnInspectCmd, nil))
	})
}

// TestLearnStatusAndResumeWithoutState proves an absent learner state is
// non-fatal: both report an empty learning picture instead of failing.
func TestLearnStatusAndResumeWithoutState(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	out := string(execute(t, learnStatusCmd, nil))
	for _, want := range []string{"not recorded", "goals: 0", "concepts: 0 recorded of 2 wiki page(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q, got:\n%s", want, out)
		}
	}
	out = string(execute(t, learnResumeCmd, nil))
	if !strings.Contains(out, "last session: none recorded") {
		t.Errorf("resume must report no session, got:\n%s", out)
	}
	if !strings.Contains(out, "due: none") {
		t.Errorf("resume must report an empty queue, got:\n%s", out)
	}
}

// TestLearnReadOnlyVerbsNeverWrite is the boundary test for phase 3: every
// read-only verb leaves no learner state behind, so running them can never
// promote anything into the record.
func TestLearnReadOnlyVerbsNeverWrite(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	for _, cmd := range []struct {
		name string
		run  func()
	}{
		{"inspect", func() { execute(t, learnInspectCmd, nil, "backend") }},
		{"status", func() { execute(t, learnStatusCmd, nil) }},
		{"resume", func() { execute(t, learnResumeCmd, nil) }},
		{"review", func() { execute(t, learnReviewCmd, nil) }},
		{"quiz", func() { execute(t, learnQuizCmd, nil, "backend/auth") }},
	} {
		t.Run(cmd.name, func(t *testing.T) {
			cmd.run()
			if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
				t.Errorf("%s created learner state", cmd.name)
			}
			if _, err := os.Stat("context/learning"); !os.IsNotExist(err) {
				t.Errorf("%s created the learning directory", cmd.name)
			}
		})
	}
}

// TestLearnStatusReportsRecordedState covers the machine format: the recorded
// concept is counted, weak and due.
func TestLearnStatusReportsRecordedState(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	learnStateFixture(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	var report learnStatusReport
	if err := json.Unmarshal(execute(t, learnStatusCmd, nil, "--format", "json"), &report); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if report.Concepts != 1 || report.Pages != 2 || report.Goals != 1 {
		t.Errorf("status counts = %+v", report)
	}
	if len(report.Weak) != 1 || report.Weak[0].ID != "backend-auth" {
		t.Errorf("a partial concept must be weak, got %+v", report.Weak)
	}
	if len(report.Due) != 1 || report.Due[0].ID != "backend-auth" {
		t.Errorf("an elapsed review must be due, got %+v", report.Due)
	}
	if report.Weak[0].Basis != learn.BasisObserved {
		t.Errorf("the reported view lost the assessment basis, got %+v", report.Weak[0])
	}
}

// learnReviewState records one due concept (auth, partial) and one weak concept
// whose review date is still ahead (registry), so both queue reasons are present.
func learnReviewState(t *testing.T, now time.Time) {
	t.Helper()
	st := &learn.State{
		Version: learn.Version,
		Profile: learn.Profile{Name: "a learner", Since: "2026-10-01"},
		Concepts: []learn.Concept{
			{
				ID: "backend-auth", Wiki: "backend/auth",
				LastAssessed: &learn.Assessment{At: "2026-10-01T09:00:00Z",
					Basis: learn.BasisObserved, Understanding: learn.UnderstandingPartial},
				NextReview: now.AddDate(0, 0, -2).Format(learn.DateLayout),
			},
			{
				ID: "backend-registry", Wiki: "backend/registry",
				LastAssessed: &learn.Assessment{At: "2026-10-01T09:00:00Z",
					Basis: learn.BasisInferred, Understanding: learn.UnderstandingWeak},
				NextReview: now.AddDate(0, 0, 7).Format(learn.DateLayout),
			},
		},
	}
	if err := learn.Save(".", st); err != nil {
		t.Fatal(err)
	}
}

// TestLearnReviewQueuesDueBeforeWeak covers the queue: an elapsed review outranks
// a weak concept, a concept that is both is queued once, and --limit bounds it.
func TestLearnReviewQueuesDueBeforeWeak(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)
	learnReviewState(t, now)

	var rows []learnReviewEntry
	if err := json.Unmarshal(execute(t, learnReviewCmd, nil, "--format", "json"), &rows); err != nil {
		t.Fatalf("review json: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want one due row and one weak row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Reason != "due" || rows[0].Concept.ID != "backend-auth" {
		t.Errorf("the elapsed review must be queued first, got %+v", rows[0])
	}
	if rows[0].DueIn != -2 {
		t.Errorf("an overdue review reports -2 days, got %d", rows[0].DueIn)
	}
	if rows[1].Reason != "weak" || rows[1].Concept.ID != "backend-registry" {
		t.Errorf("the weak concept must be queued second, got %+v", rows[1])
	}
	if rows[1].DueIn != 7 {
		t.Errorf("an upcoming review reports 7 days, got %d", rows[1].DueIn)
	}

	// --limit bounds the queue, keeping the highest-priority rows.
	out := string(execute(t, learnReviewCmd, nil, "--limit", "1"))
	if strings.Contains(out, "backend-registry") || !strings.Contains(out, "backend-auth") {
		t.Errorf("--limit must keep the due row only, got:\n%s", out)
	}
}

// TestLearnReviewDedupesOverlappingReasons proves a concept that is both due and
// weak occupies one row, not two.
func TestLearnReviewDedupesOverlappingReasons(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)
	learnStateFixture(t, now) // auth: partial AND due

	out := string(execute(t, learnReviewCmd, nil))
	if strings.Count(out, "backend-auth") != 1 {
		t.Errorf("an overlapping concept must be queued once, got:\n%s", out)
	}
}

// TestLearnQuizIsZeroHintByDefault proves the manifest carries no answer: no hint
// unless asked, and the evidence an answer must cite.
func TestLearnQuizIsZeroHintByDefault(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	var manifest learnQuizManifest
	if err := json.Unmarshal(execute(t, learnQuizCmd, nil, "backend/auth", "--format", "json"), &manifest); err != nil {
		t.Fatalf("quiz json: %v", err)
	}
	if manifest.Version != learnQuizVersion || manifest.Hinted {
		t.Errorf("a default manifest must be un-hinted, got %+v", manifest)
	}
	if len(manifest.Concepts) != 1 {
		t.Fatalf("want one concept, got %+v", manifest.Concepts)
	}
	e := manifest.Concepts[0]
	if e.Concept != "backend-auth" || e.Wiki != "backend/auth" {
		t.Errorf("manifest entry lost the concept mapping: %+v", e)
	}
	if e.Hint != "" {
		t.Errorf("zero-hint by default, got hint %q", e.Hint)
	}
	if len(e.Evidence) != 1 || e.Evidence[0] != "refs/auth.md@abcd1234:1-2" {
		t.Errorf("the manifest must cite the page evidence, got %+v", e.Evidence)
	}
	out := string(execute(t, learnQuizCmd, nil, "backend/auth"))
	if !strings.Contains(out, "hint: none") {
		t.Errorf("the text form must say the manifest is un-hinted, got:\n%s", out)
	}
}

// TestLearnQuizHintIsOptIn proves --hint is the only way a hint appears, and that
// the manifest then declares itself hinted.
func TestLearnQuizHintIsOptIn(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	var manifest learnQuizManifest
	if err := json.Unmarshal(execute(t, learnQuizCmd, nil, "backend/auth", "--hint", "--format", "json"), &manifest); err != nil {
		t.Fatalf("quiz json: %v", err)
	}
	if !manifest.Hinted {
		t.Errorf("--hint must declare the manifest hinted, got %+v", manifest)
	}
	if got := manifest.Concepts[0].Hint; got != "how a page id resolves" {
		t.Errorf("the hint must be the page summary, got %q", got)
	}
}

// TestLearnQuizValidatesManifest covers the validator: a generated manifest
// passes, and the two defects that matter are refused (a hint smuggled into a
// zero-hint manifest, an unknown schema version).
func TestLearnQuizValidatesManifest(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)

	generated := execute(t, learnQuizCmd, nil, "backend/auth", "--format", "yaml")
	if err := os.WriteFile("quiz.yaml", generated, 0o600); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, learnQuizCmd, nil, "--validate", "quiz.yaml"))
	if !strings.Contains(out, "valid") || !strings.Contains(out, "hint none") {
		t.Errorf("a generated manifest must validate, got:\n%s", out)
	}

	// A hint smuggled into a manifest that claims zero-hint is refused.
	if err := os.WriteFile("leaky.yaml", []byte("version: 1\nhinted: false\nconcepts:\n"+
		"  - concept: backend-auth\n    wiki: backend/auth\n    hint: resolution order\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnQuizCmd, nil, "--validate", "leaky.yaml"))
	})

	// An unsupported version is refused.
	if err := os.WriteFile("future.yaml", []byte("version: 99\nhinted: false\nconcepts:\n"+
		"  - concept: backend-auth\n    wiki: backend/auth\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnQuizCmd, nil, "--validate", "future.yaml"))
	})
}

// TestLearnQuizRejectsUnknownConcept proves the manifest names corpus concepts
// only, and refuses to invent one.
func TestLearnQuizRejectsUnknownConcept(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnQuizCmd, nil, "invented-concept"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnQuizCmd, nil))
	})
}

// loadRecorded reads the learner state written by a record call.
func loadRecorded(t *testing.T) *learn.State {
	t.Helper()
	st, err := learn.Load(".")
	if err != nil {
		t.Fatalf("load learner state: %v", err)
	}
	return st
}

// TestLearnRecordUpsertsConceptAndCapturesProvenance covers append-then-update:
// one record per concept, the page's relations and sources captured on creation,
// and the rating's provenance explicit (inferred unless observed is claimed).
func TestLearnRecordUpsertsConceptAndCapturesProvenance(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	out := string(execute(t, learnRecordCmd, nil,
		"--concept", "backend/auth", "--understanding", "partial"))
	if !strings.Contains(out, "created") {
		t.Errorf("the first record creates the concept, got:\n%s", out)
	}
	st := loadRecorded(t)
	if len(st.Concepts) != 1 {
		t.Fatalf("want one concept record, got %d", len(st.Concepts))
	}
	c := st.Concepts[0]
	if c.ID != "backend-auth" || c.Wiki != "backend/auth" {
		t.Errorf("the record must carry the stable id of its page, got %+v", c)
	}
	if c.LastAssessed.Basis != learn.BasisInferred {
		t.Errorf("an unclaimed basis must stay inferred (the weaker claim), got %q", c.LastAssessed.Basis)
	}
	if len(c.Prerequisites) != 1 || c.Prerequisites[0] != "backend-registry" {
		t.Errorf("the record must capture the page prerequisites, got %+v", c.Prerequisites)
	}
	if len(c.Evidence) != 1 || c.Evidence[0] != "refs/auth.md@abcd1234:1-2" {
		t.Errorf("the record must capture the page evidence, got %+v", c.Evidence)
	}
	if c.LastAssessed.At != "2026-10-13T09:00:00Z" {
		t.Errorf("the assessment must be stamped, got %q", c.LastAssessed.At)
	}

	// A second call updates the same record and may claim observed provenance.
	out = string(execute(t, learnRecordCmd, nil,
		"--concept", "backend/auth", "--understanding", "solid", "--basis", "observed",
		"--next-review", "2026-10-27"))
	if !strings.Contains(out, "updated") {
		t.Errorf("a second record updates the concept, got:\n%s", out)
	}
	st = loadRecorded(t)
	if len(st.Concepts) != 1 {
		t.Fatalf("a concept must hold one record, got %d", len(st.Concepts))
	}
	c = st.Concepts[0]
	if c.LastAssessed.Understanding != learn.UnderstandingSolid || c.LastAssessed.Basis != learn.BasisObserved {
		t.Errorf("the update must be kept, got %+v", c.LastAssessed)
	}
	if c.NextReview != "2026-10-27" {
		t.Errorf("next_review must be recorded, got %q", c.NextReview)
	}
}

// TestLearnRecordNeverImpliesMastery is the ceiling: a first assessment cannot
// claim the top level, and the refusal writes nothing.
func TestLearnRecordNeverImpliesMastery(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnRecordCmd, nil,
			"--concept", "backend/auth", "--understanding", learn.UnderstandingSolid))
	})
	if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
		t.Fatalf("a refused record must write nothing, got %v", err)
	}

	// After a real demonstration, the top level is reachable.
	execute(t, learnRecordCmd, nil, "--concept", "backend/auth", "--understanding", "partial")
	execute(t, learnRecordCmd, nil, "--concept", "backend/auth", "--understanding", "solid")
	if got := loadRecorded(t).Concepts[0].LastAssessed.Understanding; got != learn.UnderstandingSolid {
		t.Errorf("a second demonstration may reach solid, got %q", got)
	}
}

// TestLearnRecordWritesOnlyWhenAsked proves the verb refuses a content-free call
// and an unknown concept, writing nothing in either case.
func TestLearnRecordWritesOnlyWhenAsked(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	shouldExitWithCode(t, 1, func() string { return string(execute(t, learnRecordCmd, nil)) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnRecordCmd, nil, "--concept", "invented-concept", "--understanding", "weak"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnRecordCmd, nil, "--concept", "backend/auth", "--understanding", "expert"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnRecordCmd, nil, "--understanding", "weak"))
	})
	if _, err := os.Stat(learn.Path(".")); !os.IsNotExist(err) {
		t.Errorf("a refused record must leave no state, got %v", err)
	}
}

// TestLearnRecordSessionClosesTheLoop covers the session entry: appended only when
// asked, with covered and weak derived from what was just recorded.
func TestLearnRecordSessionClosesTheLoop(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	execute(t, learnRecordCmd, nil, "--profile", "a learner", "--since", "2026-10-01")
	execute(t, learnRecordCmd, nil,
		"--concept", "backend/auth", "--understanding", "partial", "--session")

	st := loadRecorded(t)
	if st.Profile.Name != "a learner" || st.Profile.Since != "2026-10-01" {
		t.Errorf("the profile must be recorded, got %+v", st.Profile)
	}
	if len(st.Sessions) != 1 {
		t.Fatalf("want one session entry, got %d", len(st.Sessions))
	}
	s := st.Sessions[0]
	if len(s.Covered) != 1 || s.Covered[0] != "backend-auth" {
		t.Errorf("the session must cover the assessed concept, got %+v", s.Covered)
	}
	if len(s.Weak) != 1 || s.Weak[0] != "backend-auth" {
		t.Errorf("a partial concept must be logged as weak, got %+v", s.Weak)
	}

	// The written session is what a resume reports.
	out := string(execute(t, learnResumeCmd, nil))
	if !strings.Contains(out, "last session: 2026-10-13T09:00:00Z") {
		t.Errorf("resume must report the recorded session, got:\n%s", out)
	}
}

// TestLearnRecordGoalUpserts covers the goal flag: one goal per concept, updated
// in place, and a malformed goal refused.
func TestLearnRecordGoalUpserts(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain the resolution order")
	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain it and its prerequisites")
	st := loadRecorded(t)
	if len(st.Goals) != 1 {
		t.Fatalf("one concept carries one goal, got %d", len(st.Goals))
	}
	if st.Goals[0].Outcome != "explain it and its prerequisites" {
		t.Errorf("a re-declared goal must be updated, got %q", st.Goals[0].Outcome)
	}
	if st.Goals[0].Created != "2026-10-13" {
		t.Errorf("a goal keeps its creation date, got %q", st.Goals[0].Created)
	}

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, learnRecordCmd, nil, "--goal", "no-outcome-here"))
	})
}

// learnTreeHash fingerprints a directory: every path with its content, so an
// added, removed or edited file changes the digest.
func learnTreeHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	var walk func(string) error
	walk = func(d string) error {
		entries, err := os.ReadDir(d)
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if e.IsDir() {
				if err := walk(p); err != nil {
					return err
				}
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\n%x\n", p, data)
		}
		return nil
	}
	if err := walk(dir); err != nil {
		t.Fatalf("fingerprint %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestLearnLaneLeavesTheWikiUntouched is the boundary test: a learner reflection
// is recorded in the learner state and nowhere else, and the wiki is byte-identical
// before and after a full session.
func TestLearnLaneLeavesTheWikiUntouched(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	before := learnTreeHash(t, sdtWikiDir)

	execute(t, learnRecordCmd, nil,
		"--profile", "a learner", "--since", "2026-10-13",
		"--goal", "backend/auth=explain the resolution order")
	execute(t, learnRecordCmd, nil, "--concept", "backend/auth", "--understanding", "partial",
		"--misconception", "thought resolution used fuzzy matching",
		"--evidence", "refs/auth.md@abcd1234:9-10", "--session")

	if after := learnTreeHash(t, sdtWikiDir); after != before {
		t.Errorf("the learning lane must leave the wiki byte-identical: %s -> %s", before, after)
	}
	st := loadRecorded(t)
	if got := st.Concepts[0].Misconception; got != "thought resolution used fuzzy matching" {
		t.Errorf("the learner reflection must be kept as an observation, got %q", got)
	}
	if len(st.Concepts[0].Evidence) != 2 {
		t.Errorf("recorded evidence must append, got %+v", st.Concepts[0].Evidence)
	}
	// The open misconception is what the learner is told to revisit, not a claim.
	var report learnStatusReport
	if err := json.Unmarshal(execute(t, learnStatusCmd, nil, "--format", "json"), &report); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if len(report.Weak) != 1 || report.Weak[0].Misconception == "" {
		t.Errorf("an open misconception must surface as weak, got %+v", report.Weak)
	}
}

// TestLearnReportsRenderEveryProfileShapeAndFormat covers the learner's identity
// shapes in the report and the yaml format of the two scope verbs.
func TestLearnReportsRenderEveryProfileShapeAndFormat(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	// No profile: the report says so rather than printing an empty name.
	if out := string(execute(t, learnResumeCmd, nil)); !strings.Contains(out, "not recorded") {
		t.Errorf("an absent profile must be reported, got:\n%s", out)
	}

	// A hand-written profile without a start date still reads as an identity:
	// the schema makes `since` optional, so the report must not assume it.
	if err := learn.Save(".", &learn.State{
		Version: learn.Version,
		Profile: learn.Profile{Name: "a learner"},
	}); err != nil {
		t.Fatal(err)
	}
	if out := string(execute(t, learnResumeCmd, nil)); !strings.Contains(out, "learner: a learner\n") {
		t.Errorf("a dateless profile must render the bare name, got:\n%s", out)
	}

	execute(t, learnRecordCmd, nil, "--profile", "a learner", "--since", "2026-10-01")
	if out := string(execute(t, learnResumeCmd, nil)); !strings.Contains(out, "a learner (since 2026-10-01)") {
		t.Errorf("a dated profile must render the start date, got:\n%s", out)
	}

	// The yaml format carries the same payload as the json one.
	var fromYAML, fromJSON learnResumeReport
	if err := yaml.Unmarshal(execute(t, learnResumeCmd, nil, "--format", "yaml"), &fromYAML); err != nil {
		t.Fatalf("resume yaml: %v", err)
	}
	if err := json.Unmarshal(execute(t, learnResumeCmd, nil, "--format", "json"), &fromJSON); err != nil {
		t.Fatalf("resume json: %v", err)
	}
	if fromYAML.Learner != fromJSON.Learner || fromYAML.Since != fromJSON.Since {
		t.Errorf("yaml and json must agree: %+v vs %+v", fromYAML, fromJSON)
	}

	var entries []learnInspectEntry
	if err := yaml.Unmarshal(execute(t, learnInspectCmd, nil, "backend", "--format", "yaml"), &entries); err != nil {
		t.Fatalf("inspect yaml: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("the yaml report must carry both concepts, got %d", len(entries))
	}
}
