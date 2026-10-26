package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// planGateFixture writes an unsplit plan whose phase checklist and completion
// criteria carry their own states, and returns its context-relative path.
func planGateFixture(t *testing.T, dir, phases, completion string) string {
	t.Helper()
	rel := "context/plan/20260101-000000-gate-plan.md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: plan
uid: 01a0e43d-a000-7000-a000-0000000000aa
status: active
updated: 2026-01-01T00:00:00Z
---

## Phase 1 — One

`+phases+`

## Completion criteria

`+completion+`
`)
	return rel
}

// TestDerivePlanOwnChecklistIgnoresCompletionCriteria is the contradiction the
// analysis found: the delivered work is the phase checklist, the completion
// boxes record intent (`plan.md`), so an unticked intent box cannot hold a
// finished plan open.
func TestDerivePlanOwnChecklistIgnoresCompletionCriteria(t *testing.T) {
	dir := runInTempDir(t)
	rel := planGateFixture(t, dir, "- [x] shipped", "- [ ] intent")
	if got := derivePlanOwnChecklist(filepath.Join(dir, rel)); got != taskFileStatusCompleted {
		t.Fatalf("derived %q, want %q", got, taskFileStatusCompleted)
	}
}

// TestDerivePlanOwnChecklistStillGatesOnPhases keeps the rule that matters: an
// open phase item still blocks the close.
func TestDerivePlanOwnChecklistStillGatesOnPhases(t *testing.T) {
	dir := runInTempDir(t)
	rel := planGateFixture(t, dir, "- [ ] open", "- [x] intent")
	if got := derivePlanOwnChecklist(filepath.Join(dir, rel)); got != ctxWikiStatusActive {
		t.Fatalf("derived %q, want %q", got, ctxWikiStatusActive)
	}
}

// TestDerivePlanOwnChecklistOnlyCompletionDerivesNothing is the graceful no-op:
// a plan with no checklist outside the intent box keeps its declared status.
func TestDerivePlanOwnChecklistOnlyCompletionDerivesNothing(t *testing.T) {
	dir := runInTempDir(t)
	rel := planGateFixture(t, dir, "", "- [ ] intent")
	if got := derivePlanOwnChecklist(filepath.Join(dir, rel)); got != "" {
		t.Fatalf("derived %q, want no derivation", got)
	}
}

// TestContextCloseUnsplAnPlanIgnoresIntentBoxes is the end-to-end gate: close
// succeeds on a finished phase checklist with the completion box still open, and
// it names the closeout slots it left to fill.
func TestContextCloseUnsplAnPlanIgnoresIntentBoxes(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	rel := "context/plan/20260101-000000-gate-plan.md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: plan
uid: 01a0e43d-a000-7000-a000-0000000000aa
analysis_id: `+cascadeUIDFor(analysis)+`
status: active
updated: 2026-01-01T00:00:00Z
sources:
  - `+analysis+`
---

## Phase 1 — One

- [x] shipped

## Completion criteria

- [ ] intent
`)
	out := string(execute(t, contextCloseCmd, nil, rel))
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, rel)), ctxMapStatus); got != taskFileStatusCompleted {
		t.Fatalf("plan status = %q, want completed", got)
	}
	if !strings.Contains(out, "closeout slots to fill:") {
		t.Errorf("close did not name the remaining slots:\n%s", out)
	}
}
