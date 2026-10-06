package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCtxHealthReport asserts each health number against a fixture corpus whose
// sources are deterministic: questions statuses, the reconciler (stale/orphans/
// drift), the citation check and the lint verdict.
func TestCtxHealthReport(t *testing.T) {
	now := reconcileNow(t)
	runInTempDir(t)

	// Questions: one unresolved (active), one resolved.
	writeCtxDoc(t, "context/questions/open.md", "---\nkind: questions\nsummary: q\nstatus: active\n---\n")
	writeCtxDoc(t, "context/questions/done.md", "---\nkind: questions\nsummary: q\nstatus: resolved\n---\n")

	// An active plan whose only task file is completed: plan/task drift.
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: active\n---\n")
	writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: done\nstatus: completed\nupdated: "+reconcileStamp(now, 1)+"\n---\n\n- [x] x <!-- c1 -->\n")
	// A second active plan with a stale in-progress task (updated 30 days before
	// the fixed clock, window 14).
	writeCtxDoc(t, "context/plan/q.md", "---\nkind: plan\nuid: uid-q\nanalysis_id: uid-a\nsummary: plan\nstatus: active\n---\n")
	writeCtxDoc(t, "context/tasks/t1.md", "---\nkind: tasks\nuid: uid-t1\nplan_id: uid-q\nsummary: open\nstatus: in-progress\nupdated: "+reconcileStamp(now, 30)+"\n---\n\n- [~] y <!-- c1 -->\n")
	// An orphan task file (no plan_id, not standalone).
	writeCtxDoc(t, "context/tasks/orphan.md", "---\nkind: tasks\nuid: uid-o\nsummary: orphan\nstatus: pending\nupdated: "+reconcileStamp(now, 1)+"\n---\n\n- [ ] z <!-- c1 -->\n")

	// One stale citation in a corpus document.
	writeTestFile(t, "context/refs/source.md", "the source body\n")
	writeCtxDoc(t, "context/notes/citing.md", "---\nkind: notes\nsummary: n\nagent: opencode\n---\nsee refs/source.md@deadbeef:1\n")

	h := ctxHealthReport(ctxStaleInProgressDays, now)
	if h.UnresolvedQuestions != 1 {
		t.Errorf("unresolved questions = %d, want 1", h.UnresolvedQuestions)
	}
	if h.StaleInProgress != 1 {
		t.Errorf("stale in-progress = %d, want 1", h.StaleInProgress)
	}
	if h.OrphanTasks != 1 {
		t.Errorf("orphan tasks = %d, want 1", h.OrphanTasks)
	}
	if h.PlanTaskDrift < 1 {
		t.Errorf("plan/task drift = %d, want >= 1", h.PlanTaskDrift)
	}
	if h.StaleCitations != 1 {
		t.Errorf("stale citations = %d, want 1", h.StaleCitations)
	}
	if h.LintWarnings < 1 {
		t.Errorf("lint warnings = %d, want >= 1", h.LintWarnings)
	}
}

// TestContextStatusHealthRender asserts the health block appears in the text
// render and as a `health` object in json, without touching the rows or the
// store verdict.
func TestContextStatusHealthRender(t *testing.T) {
	now := reconcileNow(t)
	setupContextProject(t)
	stubResumeClock(t, now)
	writeCtxDoc(t, "context/questions/open.md", "---\nkind: questions\nsummary: q\nstatus: active\n---\n")

	text := string(execute(t, contextStatusCmd, nil))
	if !strings.Contains(text, "health: unresolved 1") {
		t.Fatalf("text render must carry the health line:\n%s", text)
	}
	if !strings.Contains(text, "store:") {
		t.Fatalf("store verdict must stay in the text render:\n%s", text)
	}

	out := execute(t, contextStatusCmd, nil, "--format", "json")
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid status json: %v\n%s", err, out)
	}
	if _, ok := payload["health"]; !ok {
		t.Fatalf("json render must carry a health object: %s", out)
	}
	if _, ok := payload["store"]; !ok {
		t.Fatalf("json render must keep the store verdict: %s", out)
	}
}
