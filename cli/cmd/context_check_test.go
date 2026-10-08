package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// checkDocFixture writes a plan-shaped document with two checklist sections and
// returns its context-relative path.
func checkDocFixture(t *testing.T, dir string) string {
	t.Helper()
	rel := "context/plan/20260101-000000-plan-x.md"
	writeTestFile(t, dir+"/"+rel, `---
kind: plan
uid: 01a0e43d-a000-7000-a000-000000000001
status: active
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
project: p
---

## Phases

- [ ] one
- [ ] two

## Completion criteria

- [ ] three
`)
	return rel
}

func TestContextCheckByAnchorAcrossSections(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)

	out := execute(t, contextCheckCmd, nil, rel, "c3", "--done")
	if !strings.Contains(string(out), "3") && !strings.Contains(string(out), "c3") {
		t.Errorf("unexpected output: %q", out)
	}
	body := mustReadFile(t, dir+"/"+rel)
	if !strings.Contains(body, "- [x] three <!-- c3 -->") {
		t.Errorf("third item (second section) not ticked:\n%s", body)
	}
	if !strings.Contains(body, "updated: 2026-01-02T03:04:05Z") {
		t.Errorf("updated not refreshed:\n%s", body)
	}
}

func TestContextCheckByOrdinalFallback(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)

	execute(t, contextCheckCmd, nil, rel, "2", "--wip")
	body := mustReadFile(t, dir+"/"+rel)
	if !strings.Contains(body, "- [~] two <!-- c2 -->") {
		t.Errorf("ordinal fallback failed:\n%s", body)
	}
}

func TestContextCheckBlockReason(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)

	execute(t, contextCheckCmd, nil, rel, "c1", "--block", "--reason", "needs data")
	body := mustReadFile(t, dir+"/"+rel)
	if !strings.Contains(body, "- [!] one (blocked: needs data) <!-- c1 -->") {
		t.Errorf("block+reason failed:\n%s", body)
	}
}

func TestContextCheckErrors(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCheckCmd, nil, rel, "c9", "--done"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCheckCmd, nil, rel, "c1"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCheckCmd, nil, rel, "c1", "--done", "--wip"))
	})
}

func ambiguousCheckDocFixture(t *testing.T, dir string) string {
	t.Helper()
	rel := "context/plan/20260101-000000-plan-amb.md"
	writeTestFile(t, dir+"/"+rel, `---
kind: plan
uid: 01a0e43d-a000-7000-a000-000000000002
status: active
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
project: p
---

## Phase 1

- [ ] a1 <!-- c1 -->
- [ ] a2 <!-- c2 -->

## Phase 2

- [ ] b1 <!-- c1 -->
- [ ] b2 <!-- c2 -->
`)
	return rel
}

// TestContextCheckAmbiguousIDRefused is the end-to-end guard: `sdt context
// check` on a document whose per-section ids collide must refuse rather than
// mutate the first match.
func TestContextCheckAmbiguousIDRefused(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := ambiguousCheckDocFixture(t, dir)
	before := mustReadFile(t, dir+"/"+rel)

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCheckCmd, nil, rel, "c1", "--done"))
	})
	if after := mustReadFile(t, dir+"/"+rel); after != before {
		t.Errorf("an ambiguous id must not write:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// The ordinal fallback still ticks one specific item (the 3rd = b1).
	execute(t, contextCheckCmd, nil, rel, "3", "--done")
	if after := mustReadFile(t, dir+"/"+rel); !strings.Contains(after, "- [x] b1 <!-- c1 -->") {
		t.Errorf("ordinal fallback should tick b1:\n%s", after)
	}
}

// phaseCheckDocFixture writes a plan with two distinct `## Phase <n>` sections
// and unique ids, for the batch selector tests.

func phaseCheckDocFixture(t *testing.T, dir string) string {
	t.Helper()
	rel := "context/plan/20260101-000000-plan-phases.md"
	writeTestFile(t, dir+"/"+rel, `---
kind: plan
uid: 01a0e43d-a000-7000-a000-000000000003
status: active
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
project: p
---

## Phase 1

- [ ] alpha one <!-- c1 -->
- [ ] alpha two <!-- c2 -->

## Phase 2

- [ ] beta one <!-- c3 -->
- [ ] beta two <!-- c4 -->
`)
	return rel
}

// TestContextCheckBatchAll guards F5: `--all` ticks every item in one call.

func TestContextCheckBatchAll(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)
	out := execute(t, contextCheckCmd, nil, rel, "--done", "--all")
	if !strings.Contains(string(out), "3 item(s) updated") {
		t.Errorf("batch output = %q", out)
	}
	body := mustReadFile(t, dir+"/"+rel)
	for _, want := range []string{"- [x] one <!-- c1 -->", "- [x] two <!-- c2 -->", "- [x] three <!-- c3 -->"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in:\n%s", want, body)
		}
	}
}

// TestContextCheckBatchGrep guards F5: `--grep` ticks only matching items.

func TestContextCheckBatchGrep(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)
	execute(t, contextCheckCmd, nil, rel, "--done", "--grep", "two")
	body := mustReadFile(t, dir+"/"+rel)
	if !strings.Contains(body, "- [x] two <!-- c2 -->") {
		t.Errorf("grep did not tick 'two':\n%s", body)
	}
	if strings.Contains(body, "- [x] one") || strings.Contains(body, "- [x] three") {
		t.Errorf("grep ticked a non-matching item:\n%s", body)
	}
}

// TestContextCheckBatchPhase guards F5: `--phase <n>` scopes the batch to one
// phase section.

func TestContextCheckBatchPhase(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := phaseCheckDocFixture(t, dir)
	execute(t, contextCheckCmd, nil, rel, "--done", "--phase", "1")
	body := mustReadFile(t, dir+"/"+rel)
	if !strings.Contains(body, "- [x] alpha one <!-- c1 -->") || !strings.Contains(body, "- [x] alpha two <!-- c2 -->") {
		t.Errorf("phase 1 not fully ticked:\n%s", body)
	}
	if strings.Contains(body, "- [x] beta") {
		t.Errorf("phase batch ticked another phase:\n%s", body)
	}
}

// TestContextCheckBatchNeedsSelector guards F5: a batch with no selector is an
// error, never a silent no-op.

func TestContextCheckBatchNeedsSelector(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := checkDocFixture(t, dir)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCheckCmd, nil, rel, "--done"))
	})
}

// TestContextNewPlanStampsChecklistAnchors guards F4: a plan created from
// --input has its checklist items anchored so `check <plan> cN` works.

func TestContextNewPlanStampsChecklistAnchors(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC))
	out := execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "p", "--summary", "s",
		"--input", "## Phases\n\n- [ ] a\n- [ ] b\n")
	path := strings.TrimSpace(string(out))
	body := mustReadFile(t, filepath.Join(dir, path))
	if !strings.Contains(body, "- [ ] a <!-- c1 -->") || !strings.Contains(body, "- [ ] b <!-- c2 -->") {
		t.Errorf("plan checklist anchors not stamped:\n%s", body)
	}
}
