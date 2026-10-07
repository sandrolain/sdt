package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// resumeFixture writes a corpus with one active plan carrying a finished phase, a
// phase in progress with a blocked item, a stale task file and an orphan, then
// returns the fixed clock the fixture is built around.
func resumeFixture(t *testing.T) time.Time {
	t.Helper()
	now := reconcileNow(t)
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/done.md", "---\nkind: plan\nuid: uid-d\nsummary: finished plan\nstatus: completed\n---\n")
	writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: open\n"+
		"status: in-progress\nupdated: "+reconcileStamp(now, 20)+"\n---\n\n## Phase 1\n\n"+
		"- [x] finished item <!-- c1 -->\n\n## Phase 2\n\n"+
		"- [~] running item <!-- c2 -->\n- [!] stuck item (blocked: waiting on the user) <!-- c3 -->\n")
	writeCtxDoc(t, "context/tasks/t1.md", "---\nkind: tasks\nuid: uid-t1\nsummary: orphan\n"+
		"status: pending\nupdated: "+reconcileStamp(now, 1)+"\n---\n\n- [ ] unattached item <!-- c1 -->\n")
	return now
}

// stubResumeClock pins contextNow so the report's clock is deterministic.
func stubResumeClock(t *testing.T, now time.Time) {
	t.Helper()
	orig := contextNow
	contextNow = func() time.Time { return now }
	t.Cleanup(func() { contextNow = orig })
}

func TestBuildResumeViewRendersEverySurface(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	view := buildResumeView("", ctxStaleInProgressDays)

	if view.Premise != ctxResumePremise {
		t.Fatalf("premise = %q, want the recorded-state premise", view.Premise)
	}
	if len(view.Plans) != 1 {
		t.Fatalf("plans = %d, want only the active plan", len(view.Plans))
	}
	p := view.Plans[0]
	if p.Status != ctxWikiStatusActive {
		t.Fatalf("status = %q, want active", p.Status)
	}
	if len(p.Phases) != 2 {
		t.Fatalf("phases = %#v, want two phase sections with their counts", p.Phases)
	}
	if p.Phases[0].Label != "1" || p.Phases[0].Done != 1 || p.Phases[0].Total != 1 {
		t.Fatalf("phase 1 = %#v, want 1/1 done", p.Phases[0])
	}
	if p.Phases[1].Wip != 1 || p.Phases[1].Blocked != 1 {
		t.Fatalf("phase 2 = %#v, want one in progress and one blocked", p.Phases[1])
	}
	if len(p.Blocked) != 1 || p.Blocked[0].ID != "c3" || p.Blocked[0].Reason != "waiting on the user" {
		t.Fatalf("blocked = %#v, want the blocked item with its recorded reason", p.Blocked)
	}
	if len(view.Stale) != 1 || view.Stale[0].AgeDays != 20 {
		t.Fatalf("stale = %#v, want the 20-day-old in-progress file", view.Stale)
	}
	if len(view.Orphans) != 1 || !strings.Contains(view.Orphans[0].Reason, "no `plan_id`") {
		t.Fatalf("orphans = %#v, want the unattached task file with its reason", view.Orphans)
	}
}

func TestResumeViewSkipsCompletedPlans(t *testing.T) {
	resumeFixture(t)
	if view := buildResumeView("", ctxStaleInProgressDays); len(view.Plans) != 1 {
		t.Fatalf("plans = %#v, want the completed plan left out of the default view", view.Plans)
	}
}

func TestResumeViewExplicitPlanSelectsAnyStatus(t *testing.T) {
	resumeFixture(t)
	view := buildResumeView("context/plan/done.md", ctxStaleInProgressDays)
	if len(view.Plans) != 1 || view.Plans[0].Status != taskFileStatusCompleted {
		t.Fatalf("plans = %#v, want the completed plan selected explicitly", view.Plans)
	}
}

func TestResumeViewNoActivePlanIsEmptyNotBroken(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nsummary: plan\nstatus: completed\n---\n")
	view := buildResumeView("", ctxStaleInProgressDays)
	if len(view.Plans) != 0 || view.Stale == nil || view.Orphans == nil {
		t.Fatalf("view = %#v, want empty plans with non-nil slices so json emits []", view)
	}
}

func TestContextResumeCommandText(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	out := string(execute(t, contextResumeCmd, nil))
	for _, want := range []string{
		"plan  context/plan/p.md  [active]",
		"phase 1      1/1 done",
		"phase 2      0/2 done",
		"· 1 in progress",
		"· 1 blocked",
		"blocked c3",
		"waiting on the user",
		"stale task files",
		"orphan task files",
		ctxResumePremise,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in resume output:\n%s", want, out)
		}
	}
}

func TestContextResumeCommandJSONIsStable(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	first := string(execute(t, contextResumeCmd, nil, "--format", "json"))
	second := string(execute(t, contextResumeCmd, nil, "--format", "json"))
	if first != second {
		t.Fatalf("json output is not stable across runs:\n%s\n---\n%s", first, second)
	}
	var view resumeView
	if err := json.Unmarshal([]byte(first), &view); err != nil {
		t.Fatalf("resume json is not decodable: %v\n%s", err, first)
	}
	if view.Premise != ctxResumePremise || len(view.Plans) != 1 || len(view.Stale) != 1 || len(view.Orphans) != 1 {
		t.Fatalf("view = %#v, want every surface present in the structured output", view)
	}
	// An empty slice must marshal as [] rather than null, so a script can index it.
	if strings.Contains(first, `"orphans": null`) {
		t.Fatalf("empty orphans must marshal as [], not null:\n%s", first)
	}
}

func TestContextResumeCommandStaleDaysFlag(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	if out := string(execute(t, contextResumeCmd, nil, "--stale-days", "60")); strings.Contains(out, "stale task files") {
		t.Fatalf("expected the 60-day window to stay silent:\n%s", out)
	}
	if out := string(execute(t, contextResumeCmd, nil, "--stale-days", "5")); !strings.Contains(out, "stale task files") {
		t.Fatalf("expected the 5-day window to report the stale file:\n%s", out)
	}
}

func TestContextResumeCommandYAML(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	out := string(execute(t, contextResumeCmd, nil, "--format", "yaml"))
	for _, want := range []string{"premise:", "plans:", "stale:", "orphans:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in resume yaml:\n%s", want, out)
		}
	}
}

func TestContextResumeCommandTruncatesLongItemText(t *testing.T) {
	now := reconcileNow(t)
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: active\n---\n")
	long := strings.Repeat("word ", 40)
	writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: task\n"+
		"status: in-progress\nupdated: "+reconcileStamp(now, 0)+"\n---\n\n## Phase 1\n\n"+
		"- [!] "+strings.TrimSpace(long)+" (blocked: too long to print whole) <!-- c1 -->\n")
	stubResumeClock(t, now)
	out := string(execute(t, contextResumeCmd, nil))
	if !strings.Contains(out, "...") {
		t.Fatalf("expected the text renderer to truncate a long item:\n%s", out)
	}
	if strings.Contains(out, strings.TrimSpace(long)) {
		t.Fatalf("expected the long item not to be printed in full:\n%s", out)
	}
}

func TestContextStatusTasksRowNamesResume(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nsummary: task\nstatus: pending\n---\n\n- [ ] a <!-- c1 -->\n")
	label := func() string {
		tp, _ := ctxTypeLookup(ctxTypeTasks)
		return ctxKindLabel(tp)
	}()
	for _, row := range ctxStatusRows() {
		if row.Type != label {
			continue
		}
		// The row must point at the command that actually detects stale
		// in-progress files; it previously claimed `context status` did.
		if !strings.Contains(row.Next, "resume") {
			t.Fatalf("tasks row = %#v, want it to name `sdt context resume`", row)
		}
		return
	}
	t.Fatal("no tasks row in the status summary")
}

func TestResumePlanReferenceForms(t *testing.T) {
	// A caller should not have to know which spelling a command wants: the full
	// corpus reference, the plan/ path, a bare filename and the slug all select
	// the same plan.
	const ref = "context/plan/20261005-120000-some-plan.md"
	for _, in := range []string{ref, "plan/20261005-120000-some-plan.md", "20261005-120000-some-plan.md", "some-plan"} {
		if !resumePlanMatches(in, ref) {
			t.Errorf("resumePlanMatches(%q) must select the plan", in)
		}
	}
	for _, in := range []string{"", "   ", "another-plan", "context/plan/20260101-000000-other.md"} {
		if resumePlanMatches(in, ref) {
			t.Errorf("resumePlanMatches(%q) must not select this plan", in)
		}
	}
}

func TestResumeUnmatchedPlanSelectsNothing(t *testing.T) {
	resumeFixture(t)
	view := buildResumeView("does-not-exist.md", ctxStaleInProgressDays)
	if len(view.Plans) != 0 {
		t.Fatalf("plans = %#v, want none: an unmatched --plan must not fall back to every active plan", view.Plans)
	}
}

func TestResumeCommandAcceptsABareFilename(t *testing.T) {
	now := resumeFixture(t)
	stubResumeClock(t, now)
	out := string(execute(t, contextResumeCmd, nil, "--plan", "p.md"))
	if !strings.Contains(out, "plan  context/plan/p.md  [active]") {
		t.Fatalf("want the plan selected by its bare filename:\n%s", out)
	}
}
