package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextCloseCompletesChain(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	execute(t, contextCloseCmd, nil, task)
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, task)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("task status = %q, want completed", got)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("plan must cascade to completed, got %q", got)
	}
}

func TestContextCloseRefusesUnfinished(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, " ")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCloseCmd, nil, task))
	})
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, task)), ctxMapStatus); got != taskFileStatusInProgress {
		t.Fatalf("refused close must not write, got status %q", got)
	}
}

func TestContextCloseAllowUnfinishedRequiresReason(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, " ")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCloseCmd, nil, task, "--allow-unfinished"))
	})
}

func TestContextCloseAllowUnfinishedRecordsReason(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, " ")

	execute(t, contextCloseCmd, nil, task, "--allow-unfinished", "--reason", "deferred to a follow-up")
	body := mustReadFile(t, filepath.Join(dir, task))
	if !strings.Contains(body, "Closed with `--allow-unfinished`") || !strings.Contains(body, "deferred to a follow-up") {
		t.Fatalf("override note missing:\n%s", body)
	}
	if got := frontmatterField(body, ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("task status = %q, want completed", got)
	}
}
