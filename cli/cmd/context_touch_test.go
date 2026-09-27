package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextTouchRefreshesUpdated(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := "context/plan/20260101-000000-plan-a.md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: plan
uid: 01a0e43d-a000-7000-a000-00000000000a
status: active
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
---

## Objective

body only
`)
	out := execute(t, contextTouchCmd, nil, rel)
	if strings.TrimSpace(string(out)) != "2026-01-02T03:04:05Z" {
		t.Fatalf("touch output = %q", out)
	}
	body := mustReadFile(t, filepath.Join(dir, rel))
	if !strings.Contains(body, "updated: 2026-01-02T03:04:05Z") {
		t.Fatalf("updated not refreshed:\n%s", body)
	}
	if !strings.Contains(body, "status: active") {
		t.Fatalf("touch changed another field:\n%s", body)
	}
}

func TestContextTouchRejectsNoUpdatedKind(t *testing.T) {
	dir := runInTempDir(t)
	rel := "context/notes/20260101-000000-note.md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: notes
uid: 01a0e43d-a000-7000-a000-00000000000c
---

body
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTouchCmd, nil, rel))
	})
}
