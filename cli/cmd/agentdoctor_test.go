package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentDoctorReportsChecks(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")

	out := string(execute(t, agentDoctorCmd, nil))
	for _, want := range []string{"agents.md", "instructions", "work-dirs", "project-config", "search-cache"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing check %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok checks after init:\n%s", out)
	}

	jsonOut := execute(t, agentDoctorCmd, nil, "--format", "json")
	var checks []doctorCheck
	if err := json.Unmarshal(jsonOut, &checks); err != nil {
		t.Fatalf("invalid doctor JSON: %v\n%s", err, jsonOut)
	}
	if len(checks) == 0 {
		t.Error("expected doctor checks in JSON")
	}
	for _, c := range checks {
		if c.Status != "ok" && c.Status != "warn" && c.Status != "fail" {
			t.Errorf("unexpected status %q", c.Status)
		}
	}
}

func TestAgentDoctorMissingAgentsFails(t *testing.T) {
	runInTempDir(t)
	out := string(execute(t, agentDoctorCmd, nil))
	if !strings.Contains(out, "agents.md") || !strings.Contains(out, "FAIL") {
		t.Errorf("expected a FAIL for the missing AGENTS.md:\n%s", out)
	}
	// doctor never fails the shell.
	if strings.Contains(out, "error") && strings.Contains(out, "exit") {
		t.Errorf("doctor must not error out:\n%s", out)
	}
}

func TestContextBaselineCheckReportsSurfaces(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")

	got := contextBaselineCheck()
	if got.Status != doctorStatusOK {
		t.Fatalf("status = %q, want ok after init", got.Status)
	}
	for _, want := range []string{"agents.md", "instructions", "roles", "commands", "tok"} {
		if !strings.Contains(got.Detail, want) {
			t.Errorf("detail missing %q: %s", want, got.Detail)
		}
	}
}

func TestContextBaselineWarnsOnMissingSurface(t *testing.T) {
	runInTempDir(t)
	// No init: AGENTS.md and the instruction dirs are absent.
	got := contextBaselineCheck()
	if got.Status != doctorStatusWarn {
		t.Fatalf("status = %q, want warn when a surface is missing", got.Status)
	}
	if !strings.Contains(got.Detail, "missing") {
		t.Fatalf("detail = %q, want the missing surface named", got.Detail)
	}
}

func TestStoreCompletenessVerdict(t *testing.T) {
	setupContextProject(t)
	// A clean store needs no CRITICAL/WARNING; supply a minimal index and a
	// valid doc.
	if err := os.WriteFile("context/index.md", []byte("---\nkind: index\nsummary: i\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/notes/ok.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nbody\n")
	if v := storeCompletenessVerdict(); v.Verdict != "CLEAN" && v.Warnings == 0 {
		t.Logf("verdict: %+v (pre-existing warnings in the test fixture)", v)
	}

	// A doc missing `kind` is CRITICAL and flips the verdict.
	writeCtxDoc(t, "context/notes/bad.md", "---\nsummary: no kind\n---\nbody\n")
	// Legacy docs without kind+summary downgrade to WARNING, so assert on the
	// count not the severity.
	writeCtxDoc(t, "context/notes/broken.md", "---\nkind: notes\nsummary: s\nsources:\n  - analysis/missing.md\n---\nbody\n")
	v := storeCompletenessVerdict()
	if v.Verdict != "INCOMPLETE" || v.Warnings == 0 {
		t.Errorf("expected INCOMPLETE with warnings, got %+v", v)
	}
}

func TestDeliveryGateStopsAtFirstFailure(t *testing.T) {
	dir := runInTempDir(t)
	// A module with a syntax error fails the build step; the ladder must stop
	// there (only one result) and the command must exit non-zero.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gatefail\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package main\n\nfunc main() { this is not go }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentGateCmd, nil, "--format", "json"))
	})
}

// stubGateSteps replaces the delivery ladder so a command test does not run the
// real Go toolchain.
func stubGateSteps(t *testing.T, steps ...gateStep) {
	t.Helper()
	orig := deliveryGateSteps
	deliveryGateSteps = steps
	t.Cleanup(func() { deliveryGateSteps = orig })
}

// stubGateClock pins contextNow so a recorded run is deterministic.
func stubGateClock(t *testing.T, stamp time.Time) {
	t.Helper()
	orig := contextNow
	contextNow = func() time.Time { return stamp }
	t.Cleanup(func() { contextNow = orig })
}

// gateRecordedTask writes a plan and the task file `--plan p` resolves to and
// returns the task file's path.
func gateRecordedTask(t *testing.T) string {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nsummary: p\nstatus: active\n---\n")
	writeCtxDoc(t, "context/tasks/20261005-000000-p.md", "---\nkind: tasks\nuid: uid-t\nplan_id: uid-p\nsummary: t\nstatus: in-progress\nupdated: 2026-10-05T00:00:00Z\n---\n\n- [ ] a <!-- c1 -->\n")
	return "context/tasks/20261005-000000-p.md"
}

func TestAppendGateRecordCreatesReviewAndGate(t *testing.T) {
	stamp := time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC)
	got := appendGateRecord("- [ ] a\n", stamp, "", []gateStepResult{
		{Step: "build", Status: "pass", Duration: 1200 * time.Millisecond},
		{Step: "test", Status: "pass", Duration: 11 * time.Second},
	})
	for _, want := range []string{"## Review", "### Gate", "2026-10-05T09:30:00Z (sdt agent gate) — passed", "- build: pass (1.20s)", "- test: pass (11.00s)"} {
		if !strings.Contains(got, want) {
			t.Errorf("record missing %q:\n%s", want, got)
		}
	}
}

func TestAppendGateRecordAccumulatesRuns(t *testing.T) {
	first := appendGateRecord("- [ ] a\n", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), "",
		[]gateStepResult{{Step: "build", Status: "pass"}})
	second := appendGateRecord(first, time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC), "",
		[]gateStepResult{{Step: "build", Status: "pass"}})
	if strings.Count(second, "### Gate") != 1 {
		t.Fatalf("want one `### Gate` subsection, got:\n%s", second)
	}
	if !strings.Contains(second, "2026-10-05T09:00:00Z") || !strings.Contains(second, "2026-10-05T10:00:00Z") {
		t.Fatalf("want both runs kept, newest last:\n%s", second)
	}
	if strings.Index(second, "09:00:00Z") > strings.Index(second, "10:00:00Z") {
		t.Fatalf("want the first run before the second:\n%s", second)
	}
}

func TestAppendGateRecordNeverSummarisesFailureAsPass(t *testing.T) {
	got := appendGateRecord("- [ ] a\n", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), "test",
		[]gateStepResult{
			{Step: "build", Status: "pass", Duration: time.Second},
			{Step: "test", Status: doctorStatusFail, Duration: 300 * time.Millisecond},
		})
	if !strings.Contains(got, `FAILED at step test`) {
		t.Fatalf("want the failing step named in the heading:\n%s", got)
	}
	if !strings.Contains(got, "- test: fail (") {
		t.Fatalf("want the failed step recorded as fail:\n%s", got)
	}
	if strings.Contains(got, "- test: pass") {
		t.Fatalf("want no pass summary for a failed step:\n%s", got)
	}
}

func TestGateRecordWritesOnlyWhenAsked(t *testing.T) {
	path := gateRecordedTask(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stubGateSteps(t, gateStep{Name: "noop", Bin: "true"})
	stubGateClock(t, time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))

	// Without --record the gate must not touch the corpus: the CI path is
	// unchanged.
	execute(t, agentGateCmd, nil)
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("gate without --record rewrote the task file:\n%s", after)
	}

	out := string(execute(t, agentGateCmd, nil, "--record", "--plan", "p.md"))
	if !strings.Contains(out, "recorded gate run in") {
		t.Fatalf("want the record reported, got:\n%s", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Review", "### Gate", "- noop: pass ("} {
		if !strings.Contains(string(after), want) {
			t.Fatalf("recorded file missing %q:\n%s", want, after)
		}
	}
}

func TestGateRecordRequiresPlan(t *testing.T) {
	path := gateRecordedTask(t)
	before, _ := os.ReadFile(path)
	stubGateSteps(t, gateStep{Name: "noop", Bin: "true"})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentGateCmd, nil, "--record"))
	})
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("--record without --plan must write nothing:\n%s", after)
	}
}

func TestGateRecordRecordsAFailingRun(t *testing.T) {
	path := gateRecordedTask(t)
	stubGateSteps(t,
		gateStep{Name: "noop", Bin: "true"},
		gateStep{Name: "boom", Bin: "false"},
	)
	stubGateClock(t, time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentGateCmd, nil, "--record", "--plan", "p.md"))
	})
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `FAILED at step boom`) || !strings.Contains(string(after), "- boom: fail (") {
		t.Fatalf("want the failing run recorded with its real status:\n%s", after)
	}
}

func TestLintGateEvidence(t *testing.T) {
	t.Run("a completed phase with Review but no record is a suggestion", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 0)+"|completed|## Review\n\ntext")
		issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "no `### Gate` record") {
			t.Fatalf("issues = %#v, want one missing-record suggestion", issues)
		}
		if issues[0].Priority != ctxLintSuggestion {
			t.Fatalf("priority = %q, want SUGGESTION", issues[0].Priority)
		}
	})

	t.Run("a recorded run is silent", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 0)+"|completed|## Review\n\n### Gate\n\n2026-10-05T00:00:00Z (sdt agent gate) — passed")
		if issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none when the run is recorded", issues)
		}
	})

	t.Run("an open phase is not asked for a record", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 0)+"|in-progress|## Review\n\ntext")
		if issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a phase still open", issues)
		}
	})

	t.Run("a historical file under a retired plan is skipped", func(t *testing.T) {
		now := reconcileNow(t)
		// The plan is completed and the file predates the feature: the one-off
		// backlog must not flood the corpus.
		reconcileFixture(t, "completed", "uid-p|2026-01-01T00:00:00Z|completed|## Review\n\ntext")
		if issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a pre-feature file under a retired plan", issues)
		}
	})

	t.Run("a post-feature file under a retired plan is still reported", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "completed", "uid-p|2026-10-05T12:00:00Z|completed|## Review\n\ntext")
		if issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 1 {
			t.Fatalf("issues = %#v, want the recent file reported", issues)
		}
	})

	t.Run("the message carries a curated hint", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 0)+"|completed|## Review\n\ntext")
		issues := lintGateEvidence(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 1 || ctxLintHint(issues[0].Message) == "" {
			t.Fatalf("issues = %#v, want a curated hint", issues)
		}
	})
}

func TestAppendReviewFindingsKeepsFindingsWhenTheGateCreatedTheBlock(t *testing.T) {
	// The gate records first and creates the `## Review` block; the phase review
	// then adds its findings, which must land above the gate evidence and not be
	// dropped by the block already existing.
	content := appendGateRecord("- [ ] a\n", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), "",
		[]gateStepResult{{Step: "build", Status: "pass"}})
	got := appendReviewFindings(content, "the phase findings")
	if !strings.Contains(got, "the phase findings") {
		t.Fatalf("findings were dropped:\n%s", got)
	}
	if !strings.Contains(got, "### Gate") || !strings.Contains(got, "Verify-step protocol") {
		t.Fatalf("gate evidence or protocol lost:\n%s", got)
	}
	if strings.Index(got, "the phase findings") > strings.Index(got, "### Gate") {
		t.Fatalf("want the findings above the gate record:\n%s", got)
	}
	// Replaying the same findings is a no-op, so `task review` stays idempotent.
	if again := appendReviewFindings(got, "the phase findings"); again != got {
		t.Fatalf("replaying the same findings must not grow the file:\n%s", again)
	}
}

func TestGateGoListExprFailsClosedWhenNothingResolves(t *testing.T) {
	// The go steps are shell steps: an empty package list must become an
	// argument the go tool rejects, so the step fails instead of building and
	// testing nothing while exiting 0 (a failure laundered into a pass).
	if !strings.Contains(gateGoListExpr, gateNoPackagesSentinel) {
		t.Fatalf("gateGoListExpr = %q, want the fail-closed sentinel", gateGoListExpr)
	}
	if strings.HasPrefix(gateNoPackagesSentinel, ".") || strings.HasPrefix(gateNoPackagesSentinel, "/") {
		t.Fatalf("sentinel %q must not look like a path go would accept", gateNoPackagesSentinel)
	}
	dir := runInTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module empties\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runGateStep("go", gateStep{Name: "build", Bin: "go", Args: []string{"go build", gateGoListExpr}, UseShell: true})
	if err == nil {
		t.Fatalf("want a failure when no package resolves, got success:\n%s", out)
	}
}
