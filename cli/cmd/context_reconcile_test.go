package cmd

import (
	"strings"
	"testing"
	"time"
)

// reconcileNow is the fixed clock the staleness boundary tests compare against.
func reconcileNow(t *testing.T) time.Time {
	t.Helper()
	base, err := time.Parse(time.RFC3339, "2026-10-05T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return base
}

// reconcileStamp renders an RFC3339 `updated` value `days` before now.
func reconcileStamp(now time.Time, days int) string {
	return now.Add(-time.Duration(days) * ctxDay).UTC().Format(time.RFC3339)
}

// reconcileFixture writes an analysis, a plan and the given task files. Each task
// entry is `plan_id|updated|status|body` (pipe-separated, since the timestamp
// carries colons).
func reconcileFixture(t *testing.T, planStatus string, tasks ...string) {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: "+planStatus+"\n---\n")
	for i, spec := range tasks {
		fields := strings.SplitN(spec, "|", 4)
		for len(fields) < 4 {
			fields = append(fields, "")
		}
		planID, updated, status, body := fields[0], fields[1], fields[2], fields[3]
		path := "context/tasks/t" + string(rune('0'+i)) + ".md"
		writeCtxDoc(t, path, "---\nkind: tasks\nuid: uid-t"+string(rune('0'+i))+"\n"+
			"plan_id: "+planID+"\nsummary: task\nstatus: "+status+"\nupdated: "+updated+"\n---\n\n"+body+"\n")
	}
}

// reconcile is the report the reconciler checks and the resume view both read.
func reconcile(t *testing.T, staleDays int, now time.Time) *reconcileReport {
	t.Helper()
	return reconcileCorpus(staleDays, now)
}

func TestLintTaskOrphans(t *testing.T) {
	t.Run("absent plan_id is an orphan", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "|"+reconcileStamp(now, 0)+"|pending|- [ ] a")
		issues := lintTaskOrphans(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "no `plan_id`") {
			t.Fatalf("issues = %#v, want one absent-plan_id warning", issues)
		}
		if issues[0].Priority != ctxLintWarning {
			t.Fatalf("priority = %q, want WARNING", issues[0].Priority)
		}
		// Hints are attached corpus-wide by decorateLintHints; the check
		// guarantees only that the message class is in the curated table.
		if hint := ctxLintHint(issues[0].Message); !strings.Contains(hint, "relations backfill") {
			t.Fatalf("hint = %q, want the relations backfill remediation", hint)
		}
	})

	t.Run("unresolvable plan_id is an orphan and says so", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-missing|"+reconcileStamp(now, 0)+"|pending|- [ ] a")
		issues := lintTaskOrphans(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "does not resolve") || !strings.Contains(issues[0].Message, "uid-missing") {
			t.Fatalf("issues = %#v, want one unresolvable-plan_id warning naming the uid", issues)
		}
	})

	t.Run("resolved parent yields nothing", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 0)+"|pending|- [ ] a")
		if issues := lintTaskOrphans(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("a declared standalone file is not an orphan", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "|"+reconcileStamp(now, 0)+"|completed|"+ctxStandaloneMarker+" no parent by design")
		if issues := lintTaskOrphans(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a file that records the standalone decision", issues)
		}
	})
}

func TestLintPlanTaskStatusDrift(t *testing.T) {
	t.Run("all tasks done under an active plan", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active",
			"uid-p|"+reconcileStamp(now, 0)+"|completed|- [x] a",
			"uid-p|"+reconcileStamp(now, 0)+"|completed|- [x] b")
		issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 3 {
			t.Fatalf("issues = %d, want 3 (one plan drift + one per completed task)", len(issues))
		}
		if !strings.Contains(issues[0].Message, "all 2 task file(s) are completed") {
			t.Fatalf("plan issue = %#v, want the plan-side drift message", issues[0])
		}
		if issues[0].Priority != ctxLintSuggestion {
			t.Fatalf("priority = %q, want SUGGESTION", issues[0].Priority)
		}
		if !strings.Contains(issues[1].Message, "parent plan is still active") {
			t.Fatalf("task issue = %#v, want the task-side drift message", issues[1])
		}
	})

	t.Run("an unfinished task keeps both directions silent", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active",
			"uid-p|"+reconcileStamp(now, 0)+"|completed|- [x] a",
			"uid-p|"+reconcileStamp(now, 0)+"|in-progress|- [~] b")
		if issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none while a task is still open", issues)
		}
	})

	t.Run("an abandoned plan with completed tasks is not drift", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "abandoned", "uid-p|"+reconcileStamp(now, 0)+"|completed|- [x] a")
		if issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a terminal plan status", issues)
		}
	})

	t.Run("a completed plan is not drift", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "completed", "uid-p|"+reconcileStamp(now, 0)+"|completed|- [x] a")
		if issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("a plan with no task file yields nothing", func(t *testing.T) {
		reconcileFixture(t, "active")
		if issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, reconcileNow(t))); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("an orphan task is not counted against its would-be plan", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "|"+reconcileStamp(now, 0)+"|completed|- [x] a")
		if issues := lintPlanTaskStatusDrift(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none: the orphan belongs to no plan", issues)
		}
	})
}

func TestLintStaleInProgress(t *testing.T) {
	t.Run("exactly at the window is not stale", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, ctxStaleInProgressDays)+"|in-progress|- [~] a")
		if issues := lintStaleInProgress(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none at exactly the boundary", issues)
		}
	})

	t.Run("one day past the window is stale", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, ctxStaleInProgressDays+1)+"|in-progress|- [~] a")
		issues := lintStaleInProgress(reconcile(t, ctxStaleInProgressDays, now))
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "stale task file") || !strings.Contains(issues[0].Message, "15 day(s)") {
			t.Fatalf("issues = %#v, want one stale SUGGESTION naming the age", issues)
		}
		if issues[0].Priority != ctxLintSuggestion {
			t.Fatalf("priority = %q, want SUGGESTION", issues[0].Priority)
		}
		if ctxLintHint(issues[0].Message) == "" {
			t.Fatalf("no curated hint for %q", issues[0].Message)
		}
	})

	t.Run("the flag overrides the window", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 3)+"|in-progress|- [~] a")
		if issues := lintStaleInProgress(reconcile(t, 2, now)); len(issues) != 1 {
			t.Fatalf("issues = %#v, want the 2-day window to fire on a 3-day-old file", issues)
		}
		if issues := lintStaleInProgress(reconcile(t, 30, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want the 30-day window to stay silent", issues)
		}
	})

	t.Run("the legacy active status is covered too", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, 40)+"|active|- [~] a")
		if issues := lintStaleInProgress(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 1 {
			t.Fatalf("issues = %#v, want the legacy status covered", issues)
		}
	})

	t.Run("a completed or pending file is never stale", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active",
			"uid-p|"+reconcileStamp(now, 40)+"|completed|- [x] a",
			"uid-p|"+reconcileStamp(now, 40)+"|pending|- [ ] b")
		if issues := lintStaleInProgress(reconcile(t, ctxStaleInProgressDays, now)); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("an unparseable or absent updated is skipped", func(t *testing.T) {
		runInTempDir(t)
		writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: task\nstatus: in-progress\n---\n")
		writeCtxDoc(t, "context/tasks/t1.md", "---\nkind: tasks\nuid: uid-t1\nplan_id: uid-p\nsummary: task\nstatus: in-progress\nupdated: yesterday\n---\n")
		issues := lintStaleInProgress(reconcile(t, ctxStaleInProgressDays, reconcileNow(t)))
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none: the timestamp lint reports the malformed value", issues)
		}
	})

	t.Run("a non-positive window falls back to the default", func(t *testing.T) {
		now := reconcileNow(t)
		reconcileFixture(t, "active", "uid-p|"+reconcileStamp(now, ctxStaleInProgressDays+1)+"|in-progress|- [~] a")
		if issues := lintStaleInProgress(reconcile(t, 0, now)); len(issues) != 1 {
			t.Fatalf("issues = %#v, want the default window applied", issues)
		}
	})
}

func TestReconcileCorpusReadsOnlyTaskAndPlanDocuments(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nsummary: plan\nstatus: active\n---\n")
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nuid: uid-n\nsummary: note\n---\n")
	rep := reconcile(t, ctxStaleInProgressDays, reconcileNow(t))
	if len(rep.Tasks) != 0 {
		t.Fatalf("tasks = %#v, want none: only `kind: tasks` documents are reconciled", rep.Tasks)
	}
	if len(rep.Plans) != 1 {
		t.Fatalf("plans = %d, want only the plan document", len(rep.Plans))
	}
}

func TestReconcilePhases(t *testing.T) {
	content := "---\nkind: tasks\n---\n\n## Phase 1\n\n- [x] a <!-- c1 -->\n- [ ] b <!-- c2 -->\n\n" +
		"## Phase 10\n\n- [~] c <!-- c3 -->\n- [!] d (blocked: waiting on the user) <!-- c4 -->\n"
	phases, blocked := reconcilePhases(content)
	if len(phases) != 2 {
		t.Fatalf("phases = %#v, want two phase sections", phases)
	}
	if phases[0].Label != "1" || phases[0].Done != 1 || phases[0].Total != 2 {
		t.Fatalf("phase 1 = %#v, want label 1 with 1/2 done", phases[0])
	}
	if phases[1].Label != "10" || phases[1].Wip != 1 || phases[1].Blocked != 1 {
		t.Fatalf("phase 10 = %#v, want label 10 with one wip and one blocked item", phases[1])
	}
	if len(blocked) != 1 || blocked[0].ID != "c4" || blocked[0].Reason != "waiting on the user" {
		t.Fatalf("blocked = %#v, want the id and the recorded reason", blocked)
	}
	if strings.Contains(blocked[0].Text, "blocked:") {
		t.Fatalf("text = %q, want the reason suffix stripped from the text", blocked[0].Text)
	}
}

func TestReconcilePhasesNumbersPhasesNumerically(t *testing.T) {
	// A lexical sort would put phase 10 before phase 2.
	content := "---\nkind: tasks\n---\n\n## Phase 2\n\n- [ ] a\n\n## Phase 10\n\n- [ ] b\n"
	phases, _ := reconcilePhases(content)
	if len(phases) != 2 || phases[0].Label != "2" || phases[1].Label != "10" {
		t.Fatalf("phases = %#v, want 2 before 10", phases)
	}
}

func TestReconcilePhasesCountsPreambleItems(t *testing.T) {
	content := "---\nkind: tasks\n---\n\n- [ ] before\n\n## Phase 1\n\n- [ ] inside\n"
	phases, _ := reconcilePhases(content)
	if len(phases) != 2 {
		t.Fatalf("phases = %#v, want the preamble accounted for", phases)
	}
	if phases[0].Label != "" || phases[0].Total != 1 {
		t.Fatalf("preamble = %#v, want one unlabelled item", phases[0])
	}
}
