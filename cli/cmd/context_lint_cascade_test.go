package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestLintCascadeDriftDirections(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	// Task declared completed with an open item + analysis declared completed
	// with an open plan.
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", taskFileStatusCompleted)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, " ")

	issues := lintCascadeDrift()
	msgs := []string{}
	for _, i := range issues {
		msgs = append(msgs, i.Message)
		if i.Priority != ctxLintWarning {
			t.Fatalf("cascade drift must be advisory WARNING, got %q", i.Priority)
		}
	}
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "task declares completed but its checklist") {
		t.Fatalf("missing task drift:\n%s", joined)
	}
	if !strings.Contains(joined, "analysis declares completed but") {
		t.Fatalf("missing analysis drift:\n%s", joined)
	}
}

func TestLintCascadeDriftAdoption(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")
	// A task whose checklist is complete while its status is not.
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-2", taskFileStatusPending, plan, "x")

	joined := ""
	for _, i := range lintCascadeDrift() {
		joined += i.Message + "\n"
	}
	if !strings.Contains(joined, "task checklist is complete but the file status") {
		t.Fatalf("missing task adoption:\n%s", joined)
	}
	if !strings.Contains(joined, "analysis is derivably completed") {
		t.Fatalf("missing analysis adoption:\n%s", joined)
	}
}

func TestLintCascadeDriftCleanTree(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", taskFileStatusCompleted)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")

	if issues := lintCascadeDrift(); len(issues) != 0 {
		t.Fatalf("clean tree must yield no drift, got %#v", issues)
	}
}

// TestLintCascadeDriftLeavesTheObjectivesCaseToItsOwnCheck pins the boundary
// between the two checks once the derivation gates on the objectives checklist:
// an analysis whose plans are all done but whose objectives are unticked is
// reported by lintObjectiveLifecycle, and the cascade lint stays quiet so one
// disagreement does not become two messages on one file.
func TestLintCascadeDriftLeavesTheObjectivesCaseToItsOwnCheck(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysisObjectives(t, dir, "20260101-000000-a", taskFileStatusCompleted, "- [x] O1 shipped\n- [ ] O2 owed\n")
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", taskFileStatusCompleted, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusCompleted, plan, "x")

	for _, i := range lintCascadeDrift() {
		t.Errorf("the cascade lint must not report this file once the objectives gate owns it: %s", i.Message)
	}

	ref := normalizeContextRef(analysis)
	life := lintObjectiveLifecycle(ref, analysis, objectiveCoverage{
		AnalysisRef:    ref,
		AnalysisStatus: taskFileStatusCompleted,
		objectives: []analysisObjective{
			{ID: "O1", Done: true},
			{ID: "O2", Done: false},
		},
	})
	if len(life) != 1 {
		t.Fatalf("the lifecycle check must own exactly one finding, got %#v", life)
	}
	if !strings.Contains(life[0].Message, "1 of 2 objectives unticked") {
		t.Fatalf("unexpected lifecycle message: %s", life[0].Message)
	}
}

func TestDerivedCompletionBlock(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, " ")

	// A plan with an open child must block a declared `completed`.
	derived, reason := derivedCompletionBlock(plan)
	if derived != ctxWikiStatusActive || reason == "" {
		t.Fatalf("expected the guard to block, got derived=%q reason=%q", derived, reason)
	}
	// Deriving `completed` (all children done) must not block.
	writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")
	if _, err := cascadeUp(normalizeContextRef(plan), true); err != nil {
		t.Fatal(err)
	}
	if derived, reason := derivedCompletionBlock(plan); reason != "" {
		t.Fatalf("a derivably completed plan must not block, got derived=%q reason=%q", derived, reason)
	}
}
