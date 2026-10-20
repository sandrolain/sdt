package cmd

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cascadeFixedTime = "2026-01-01T00:00:00Z"

// cascadeUIDFor derives a stable, distinct uid per fixture document from its
// corpus reference: the typed relation is resolved through uids, so two
// documents of the same fixture must not share one.
func cascadeUIDFor(rel string) string {
	sum := sha256.Sum256([]byte(rel))
	return fmt.Sprintf("01a0e43d-a000-7000-a000-%012x", sum[:6])
}

func writeCascadeAnalysis(t *testing.T, dir, name, status string) string {
	t.Helper()
	rel := "context/analysis/" + name + ".md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: analysis
uid: `+cascadeUIDFor(rel)+`
title: a
status: `+status+`
updated: `+cascadeFixedTime+`
---
`)
	return rel
}

func writeCascadePlan(t *testing.T, dir, name, status, analysisRel string, ownDone bool) string {
	t.Helper()
	rel := "context/plan/" + name + ".md"
	mark := " "
	if ownDone {
		mark = "x"
	}
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: plan
uid: `+cascadeUIDFor(rel)+`
analysis_id: `+cascadeUIDFor(analysisRel)+`
status: `+status+`
updated: `+cascadeFixedTime+`
sources:
  - `+analysisRel+`
---

## Phases

- [`+mark+`] do it
`)
	return rel
}

func writeCascadeTask(t *testing.T, dir, name, status, planRel string, mark string) string {
	t.Helper()
	rel := "context/tasks/" + name + ".md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: tasks
uid: `+cascadeUIDFor(rel)+`
plan_id: `+cascadeUIDFor(planRel)+`
status: `+status+`
updated: `+cascadeFixedTime+`
sources:
  - `+planRel+`
---

- [`+mark+`] step
`)
	return rel
}

func TestCascadeFullTreeCompletes(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	changes, err := cascadeUp(normalizeContextRef(task), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("expected task→plan→analysis flips, got %+v", changes)
	}
	if changes[0].To != taskFileStatusCompleted || changes[1].To != taskFileStatusCompleted || changes[2].To != taskFileStatusCompleted {
		t.Fatalf("unexpected chain: %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("plan status = %q", got)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("analysis status = %q", got)
	}
	again, err := cascadeUp(normalizeContextRef(task), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second cascade must be a no-op, got %+v", again)
	}
}

func TestCascadePartialStaysActive(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "~")

	changes, err := cascadeUp(normalizeContextRef(task), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("open task must not flip anything, got %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("analysis status = %q", got)
	}
}

// TestCascadePlanOwnChecklistIsReferenceOnly guards F13: the plan's own `##
// Phases` checklist is intent, not a second executed checklist, so a plan with
// an open own checklist still completes from its completed task files.

func TestCascadePlanOwnChecklistIsReferenceOnly(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	if _, err := cascadeUp(normalizeContextRef(task), true); err != nil {
		t.Fatal(err)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("plan must complete from its task file despite an open own checklist, got %q", got)
	}
}

func TestCascadeSkipsNoChildrenAndAbandoned(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	// An unsplit plan derives from its own checklist: this fixture's checklist is
	// ticked, so the plan and its analysis complete without any task file.
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	changes, err := cascadeUp(normalizeContextRef(plan), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("a plan whose own checklist is done must complete, cascading to its analysis, got %+v", changes)
	}
	// A plan with no checklist at all has nothing to derive from: it keeps its
	// declared status.
	untracked := writeCascadePlan(t, dir, "20260101-000000-a-nolist", ctxWikiStatusActive, analysis, true)
	content := mustReadFile(t, filepath.Join(dir, untracked))
	writeTestFile(t, filepath.Join(dir, untracked), strings.Replace(content, "- [x] do it\n", "", 1))
	changes, err = cascadeUp(normalizeContextRef(untracked), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("a plan with no checklist must not flip, got %+v", changes)
	}
	// An abandoned plan never counts as completion for its analysis.
	analysis2 := writeCascadeAnalysis(t, dir, "20260101-000002-b", ctxWikiStatusActive)
	plan2 := writeCascadePlan(t, dir, "20260101-000002-b-plan", "abandoned", analysis2, true)
	if _, err := cascadeUp(normalizeContextRef(plan2), true); err != nil {
		t.Fatal(err)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis2)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("analysis must stay active while a plan is abandoned, got %q", got)
	}
	if !strings.Contains(mustReadFile(t, filepath.Join(dir, plan2)), "status: abandoned") {
		t.Fatal("abandoned plan must not be rewritten")
	}
}

func TestCascadeOnePlanOfTwoBlocks(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan1 := writeCascadePlan(t, dir, "20260101-000000-a-plan1", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan1-phase-1", taskFileStatusCompleted, plan1, "x")
	plan2 := writeCascadePlan(t, dir, "20260101-000000-a-plan2", ctxWikiStatusActive, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan2-phase-1", taskFileStatusInProgress, plan2, " ")

	changes, err := cascadeUp(normalizeContextRef(analysis), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("analysis with one open plan must not flip, got %+v", changes)
	}
}

// writeCascadeAnalysisObjectives writes an analysis that declares an
// `## Objectives` checklist, the section the derivation gates on.
func writeCascadeAnalysisObjectives(t *testing.T, dir, name, status, objectives string) string {
	t.Helper()
	rel := "context/analysis/" + name + ".md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: analysis
uid: `+cascadeUIDFor(rel)+`
title: a
status: `+status+`
updated: `+cascadeFixedTime+`
---

## Objectives

`+objectives)
	return rel
}

// completedAnalysisTree builds the shape the defect appeared in: an analysis
// declaring two objectives and one completed plan whose task file is done.
func completedAnalysisTree(t *testing.T, objectives string) (string, string) {
	t.Helper()
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysisObjectives(t, dir, "20260101-000000-a", ctxWikiStatusActive, objectives)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")
	return dir, analysis
}

// TestCascadeAnalysisObjectivesBlockCompletion is the defect this gate exists
// for. The plan edge said "the derived work is done" and the cascade closed the
// whole analysis on it, so a plan covering one of two objectives completed an
// analysis that still owed the other — and the lifecycle check then reported
// the drift the cascade had created.
func TestCascadeAnalysisObjectivesBlockCompletion(t *testing.T) {
	dir, analysis := completedAnalysisTree(t, "- [x] O1 shipped by the plan\n- [ ] O2 still owed\n")

	changes, err := cascadeUp(normalizeContextRef(analysis), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("an analysis whose checklist denies completion must not flip, got %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("analysis status = %q, want it left active", got)
	}
}

// TestCascadeAnalysisObjectivesCompleteStillCloses pins the other direction: the
// gate refuses a completion the checklist denies, it does not refuse every
// completion. An analysis whose plans are done and whose objectives are all
// ticked still closes.
func TestCascadeAnalysisObjectivesCompleteStillCloses(t *testing.T) {
	dir, analysis := completedAnalysisTree(t, "- [x] O1 shipped\n- [x] O2 shipped\n")

	changes, err := cascadeUp(normalizeContextRef(analysis), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].To != taskFileStatusCompleted {
		t.Fatalf("expected one analysis flip to completed, got %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("analysis status = %q", got)
	}
}

// TestCascadeAnalysisObjectivesAbsentIsUnchanged guards backward compatibility
// and the boundary of the gate: only the `## Objectives` checklist is read, so
// an analysis without the section — and one carrying an unticked checklist
// somewhere else — derives from its plans alone.
func TestCascadeAnalysisObjectivesAbsentIsUnchanged(t *testing.T) {
	dir, analysis := completedAnalysisTree(t, "- [ ] O1 declared but this is not the section\n")
	// Move the checklist out of `## Objectives`: it is a note, not objectives.
	path := filepath.Join(dir, analysis)
	body := strings.Replace(mustReadFile(t, path), "## Objectives", "## Context", 1)
	writeTestFile(t, path, body)

	changes, err := cascadeUp(normalizeContextRef(analysis), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].To != taskFileStatusCompleted {
		t.Fatalf("an analysis without `## Objectives` derives from its plans, got %+v", changes)
	}
}

// TestCascadeAnalysisObjectivesMalformedIsIgnored keeps the gate from becoming a
// new failure mode: a checklist line whose id is not leading declares no
// objective, so it cannot block a derivation.
func TestCascadeAnalysisObjectivesMalformedIsIgnored(t *testing.T) {
	_, analysis := completedAnalysisTree(t, "- [ ] see O1 for details\n- [ ] no id at all\n")

	changes, err := cascadeUp(normalizeContextRef(analysis), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].To != taskFileStatusCompleted {
		t.Fatalf("a non-declaring checklist must not block, got %+v", changes)
	}
}
