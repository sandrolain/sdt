package cmd

import (
	"bytes"
	"os"
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

	var stderr bytes.Buffer
	prevErr := rootCmd.ErrOrStderr()
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() { rootCmd.SetErr(prevErr) })

	// A refusal is a decision for the caller, so it is exit 4 and it names both
	// ways forward instead of reporting a failure.
	shouldExitWithCode(t, 4, func() string {
		return string(execute(t, contextCloseCmd, nil, task))
	})
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, task)), ctxMapStatus); got != taskFileStatusInProgress {
		t.Fatalf("refused close must not write, got status %q", got)
	}
	out := stderr.String()
	for _, want := range []string{
		"sdt-decision reason=unfinished",
		"sdt context resume tasks/",
		"sdt context close tasks/",
		"--allow-unfinished --reason",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("decision output missing %q:\n%s", want, out)
		}
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

// TestContextClosePrintsSelfAssessmentInvocation guards the answered gap:
// closing a plan hands the reader the `context new` command the mandatory
// self-assessment note needs, and writes nothing itself.

func TestContextClosePrintsSelfAssessmentInvocation(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)

	out := string(execute(t, contextCloseCmd, nil, plan))
	for _, want := range []string{
		"self-assessment: sdt context new",
		"--type notes",
		"--note-type self-assessment",
		"--slug a-plan-workflow-self-assessment",
		"--source plan/20260101-000000-a-plan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("close output lacks %q:\n%s", want, out)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "context/notes")); err == nil && len(entries) > 0 {
		t.Errorf("close wrote %d note(s); it must only print the invocation", len(entries))
	}
}

// TestContextCloseTaskPrintsNoSelfAssessment keeps the line plan-only: a task
// close has no note of its own.

func TestContextCloseTaskPrintsNoSelfAssessment(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, false)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	out := string(execute(t, contextCloseCmd, nil, task))
	if strings.Contains(out, "self-assessment:") {
		t.Errorf("task close printed a self-assessment line:\n%s", out)
	}
}

// TestPlanSelfAssessmentInvocation pins the command's shape: the title is
// quoted, the slug drops the dated prefix, and agent/objective appear only when
// the plan carries them.

func TestPlanSelfAssessmentInvocation(t *testing.T) {
	content := "---\ntitle: \"A plan\"\nagent: opencode\nobjective: ctx\n---\n"
	got := planSelfAssessmentInvocation("context/plan/20260101-000000-a-plan.md", content)
	for _, want := range []string{
		`--title "A plan — workflow self-assessment"`,
		"--slug a-plan-workflow-self-assessment",
		"--agent opencode",
		"--objective ctx",
		"--source plan/20260101-000000-a-plan",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("invocation lacks %q:\n%s", want, got)
		}
	}

	bare := planSelfAssessmentInvocation("context/plan/20260101-000000-a-plan.md", "---\nkind: plan\n---\n")
	if strings.Contains(bare, "--agent") || strings.Contains(bare, "--objective") {
		t.Errorf("a plan without agent/objective must omit both flags:\n%s", bare)
	}
	if !strings.Contains(bare, `--title "a-plan — workflow self-assessment"`) {
		t.Errorf("a plan without a title falls back to its slug:\n%s", bare)
	}
}
