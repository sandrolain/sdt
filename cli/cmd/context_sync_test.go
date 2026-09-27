package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncAdvancesDerivedStatus(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")

	changes, err := syncStore(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind != ctxTypeAnalysis || changes[0].To != taskFileStatusCompleted {
		t.Fatalf("expected the analysis to advance, got %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("analysis status = %q", got)
	}
	again, err := syncStore(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second sync must be a no-op, got %+v", again)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")

	changes, err := syncStore(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("dry-run should report the change, got %+v", changes)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("dry-run must not write, analysis status = %q", got)
	}
}

func TestSyncNothingToDo(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	changes, err := syncStore(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("childless analysis must not flip, got %+v", changes)
	}
}

func TestSyncDocScope(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")

	out := execute(t, contextSyncCmd, nil, "--doc", plan)
	if !strings.Contains(string(out), plan) {
		t.Fatalf("scoped sync should report the plan flip, got %q", out)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("plan status = %q", got)
	}
}
