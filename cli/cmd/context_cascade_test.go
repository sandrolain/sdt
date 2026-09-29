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

func TestCascadePlanOwnChecklistBlocks(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	if _, err := cascadeUp(normalizeContextRef(task), true); err != nil {
		t.Fatal(err)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("plan with open own checklist must stay active, got %q", got)
	}
}

func TestCascadeSkipsNoChildrenAndAbandoned(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	// A plan with no task file is never auto-flipped.
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	changes, err := cascadeUp(normalizeContextRef(plan), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("childless plan must not flip, got %+v", changes)
	}
	// An abandoned plan never counts as completion for the analysis.
	plan2 := writeCascadePlan(t, dir, "20260101-000000-a-plan2", "abandoned", analysis, true)
	if _, err := cascadeUp(normalizeContextRef(plan2), true); err != nil {
		t.Fatal(err)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != ctxWikiStatusActive {
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
