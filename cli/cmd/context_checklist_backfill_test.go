package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func checklistBackfillFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, filepath.Join(dir, "context/plan/20260101-000000-plan-a.md"), `---
kind: plan
uid: 01a0e43d-a000-7000-a000-00000000000a
status: active
updated: 2026-01-01T00:00:00Z
---

## Phases

- [ ] one
- [ ] two
`)
	writeTestFile(t, filepath.Join(dir, "context/tasks/20260101-000000-plan-a-phase-1.md"), `---
kind: tasks
uid: 01a0e43d-a000-7000-a000-00000000000b
phase: 1
status: pending
updated: 2026-01-01T00:00:00Z
---

- [x] already <!-- c2 -->
- [ ] fresh
`)
}

func TestChecklistBackfillDryRunThenWrite(t *testing.T) {
	dir := runInTempDir(t)
	checklistBackfillFixture(t, dir)

	res, err := stampChecklistFiles(true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stamped != 2 || res.Action != statusDryRun {
		t.Fatalf("dry-run = %+v", res)
	}
	if strings.Contains(mustReadFile(t, filepath.Join(dir, "context/plan/20260101-000000-plan-a.md")), "<!-- c") {
		t.Fatal("dry-run must not write")
	}
	if checklistBackfillDone() {
		t.Fatal("dry-run must not write the marker")
	}

	res, err = stampChecklistFiles(false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stamped != 2 || res.Action != statusWritten {
		t.Fatalf("write = %+v", res)
	}
	plan := mustReadFile(t, filepath.Join(dir, "context/plan/20260101-000000-plan-a.md"))
	if !strings.Contains(plan, "- [ ] one <!-- c1 -->") || !strings.Contains(plan, "- [ ] two <!-- c2 -->") {
		t.Fatalf("plan not stamped:\n%s", plan)
	}
	task := mustReadFile(t, filepath.Join(dir, "context/tasks/20260101-000000-plan-a-phase-1.md"))
	if !strings.Contains(task, "- [x] already <!-- c2 -->") || !strings.Contains(task, "- [ ] fresh <!-- c3 -->") {
		t.Fatalf("task not stamped monotonically:\n%s", task)
	}
	if !checklistBackfillDone() {
		t.Fatal("real run must write the marker")
	}
}

func TestChecklistBackfillIdempotent(t *testing.T) {
	dir := runInTempDir(t)
	checklistBackfillFixture(t, dir)

	if _, err := stampChecklistFiles(false); err != nil {
		t.Fatal(err)
	}
	res, err := stampChecklistFiles(false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stamped != 0 {
		t.Fatalf("second run must be a no-op, got %+v", res)
	}
}
