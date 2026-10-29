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
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("a closed task must leave the plan active, got %q", got)
	}
	// The plan is the writer of its own completion, and of the closeout that goes
	// with it: closing it is the act that flips the analysis too.
	execute(t, contextCloseCmd, nil, plan)
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, plan)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("closing the plan must complete it, got %q", got)
	}
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, analysis)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("closing the plan must cascade to the analysis, got %q", got)
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

// TestTaskReviewTargetsAnUnsplitPlan: a plan that was never split has no task
// file, and its review block belongs in the plan itself. The plan's status is
// not written by the review — `close` owns it.
func TestTaskReviewTargetsAnUnsplitPlan(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)

	out := string(execute(t, contextTaskReviewCmd, nil, "--plan", plan, "--input", "the suite ran green (CONFIRMED)"))
	if !strings.Contains(out, plan) {
		t.Fatalf("the review was not written to the plan: %q", out)
	}
	body := mustReadFile(t, filepath.Join(dir, plan))
	if !strings.Contains(body, "## Review") || !strings.Contains(body, "the suite ran green") {
		t.Fatalf("the plan carries no review block:\n%s", body)
	}
	if got := frontmatterField(body, ctxMapStatus); got != ctxWikiStatusActive {
		t.Fatalf("the review must not complete the plan, got %q", got)
	}
}
