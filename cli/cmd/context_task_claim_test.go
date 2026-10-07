package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestTaskClaimReleaseRoundTrip(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nsummary: p\nstatus: active\n---\n")
	writeCtxDoc(t, "context/tasks/20261005-000000-p.md", "---\nkind: tasks\nuid: uid-t\nplan_id: uid-p\nsummary: t\nstatus: pending\nupdated: 2026-10-05T00:00:00Z\n---\n\n- [ ] a <!-- c1 -->\n")

	execute(t, contextTaskClaimCmd, nil, "--plan", "p.md", "--agent", "tester")
	content := mustReadFile(t, "context/tasks/20261005-000000-p.md")
	if !strings.Contains(content, "claimed_by: tester") {
		t.Fatalf("claim must record claimed_by:\n%s", content)
	}
	if !strings.Contains(content, "claimed_at: 2026-10-07T12:00:00Z") {
		t.Fatalf("claim must record claimed_at from the clock:\n%s", content)
	}
	if !strings.Contains(content, "status: in-progress") {
		t.Fatalf("claim must flip the file in progress:\n%s", content)
	}

	execute(t, contextTaskReleaseCmd, nil, "--plan", "p.md")
	content = mustReadFile(t, "context/tasks/20261005-000000-p.md")
	if strings.Contains(content, "claimed_by") || strings.Contains(content, "claimed_at") {
		t.Fatalf("release must clear the claim:\n%s", content)
	}
	if !strings.Contains(content, "status: pending") {
		t.Fatalf("release must set the file back to pending:\n%s", content)
	}
}

func TestResumeFrontierExcludesClaimedAndCompleted(t *testing.T) {
	runInTempDir(t)
	now := reconcileNow(t)
	stubResumeClock(t, now)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: p\nstatus: active\n---\n")
	// Claimable: pending, unclaimed.
	writeCtxDoc(t, "context/tasks/20261005-000000-p.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: t\nstatus: pending\nupdated: 2026-10-05T00:00:00Z\n---\n\n- [ ] a <!-- c1 -->\n")
	// Taken: claimed by bob.
	writeCtxDoc(t, "context/tasks/20261005-010000-p.md", "---\nkind: tasks\nuid: uid-t1\nplan_id: uid-p\nsummary: t\nstatus: in-progress\nupdated: 2026-10-05T00:00:00Z\nclaimed_by: bob\nclaimed_at: 2026-10-05T00:00:00Z\n---\n\n- [x] a <!-- c1 -->\n")

	view := buildResumeView("", ctxStaleInProgressDays)
	if len(view.Frontier) != 1 || view.Frontier[0].Task != "context/tasks/20261005-000000-p.md" {
		t.Fatalf("frontier = %#v, want only the pending unclaimed file", view.Frontier)
	}
}

func TestResumeReportsStaleClaim(t *testing.T) {
	runInTempDir(t)
	now := reconcileNow(t) // 2026-10-05
	stubResumeClock(t, now)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: p\nstatus: active\n---\n")
	// Claimed 30 days before the fixed clock -> past the 14-day window.
	writeCtxDoc(t, "context/tasks/20261005-000000-p.md", "---\nkind: tasks\nuid: uid-t\nplan_id: uid-p\nsummary: t\nstatus: in-progress\nupdated: 2026-09-05T00:00:00Z\nclaimed_by: bob\nclaimed_at: 2026-09-05T00:00:00Z\n---\n\n- [ ] a <!-- c1 -->\n")

	view := buildResumeView("", ctxStaleInProgressDays)
	if len(view.StaleClaims) != 1 || view.StaleClaims[0].ClaimedBy != "bob" || view.StaleClaims[0].AgeDays < 14 {
		t.Fatalf("stale claims = %#v, want the 30-day-old claim reported", view.StaleClaims)
	}
}
