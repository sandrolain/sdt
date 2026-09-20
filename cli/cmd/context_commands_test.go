package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestContextCommandsNew verifies the trigger stub creation and index
// regeneration from a directory scan (user triggers listed, format identical
// to `sdt agent init`).
func TestContextCommandsNew(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 17, 30, 0, 0, time.UTC))

	out := string(execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research"))
	if !strings.Contains(out, "context/commands/triage.md") {
		t.Errorf("missing new file in output: %q", out)
	}
	if !strings.Contains(out, "regenerated context/commands/index.md") {
		t.Errorf("missing index regen in output: %q", out)
	}

	stub := mustReadFile(t, "context/commands/triage.md")
	if got := frontmatterField(stub, "kind"); got != "commands" {
		t.Errorf("stub kind = %q", got)
	}
	if got := frontmatterField(stub, "id"); got != "commands/triage" {
		t.Errorf("stub id = %q", got)
	}
	if !strings.Contains(stub, "context/instructions/research.md") {
		t.Errorf("stub must reference the --contract instruction:\n%s", stub)
	}

	idx := mustReadFile(t, "context/commands/index.md")
	if !strings.Contains(idx, ">triage") {
		t.Errorf("index missing >triage row:\n%s", idx)
	}
	if _, err := os.Stat("context/commands/index.md"); err != nil {
		t.Fatalf("index missing: %v", err)
	}
}

// TestContextCommandsNewRejects guards the trigger grammer and the reserved
// index name.
func TestContextCommandsNewRejects(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "bad/slug"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "index"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "bad/contract"))
	})
}

// TestContextCommandsRm verifies the archive move and index regeneration.
func TestContextCommandsRm(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 17, 31, 0, 0, time.UTC))
	writeTestFile(t, "context/commands/triage.md", buildCommandsStub("triage"))

	out := string(execute(t, contextCommandsRmCmd, nil, "triage"))
	if !strings.Contains(out, "context/archive/20260920-173100-commands-triage.md") {
		t.Errorf("missing archive path in output: %q", out)
	}
	if !strings.Contains(out, "regenerated context/commands/index.md") {
		t.Errorf("missing index regen in output: %q", out)
	}
	if _, err := os.Stat("context/commands/triage.md"); !os.IsNotExist(err) {
		t.Errorf("trigger still present: %v", err)
	}
	archived := mustReadFile(t, "context/archive/20260920-173100-commands-triage.md")
	if got := frontmatterField(archived, "status"); got != "archived" {
		t.Errorf("archived status = %q", got)
	}
	idx := mustReadFile(t, "context/commands/index.md")
	if strings.Contains(idx, ">triage") {
		t.Errorf("index still lists removed trigger:\n%s", idx)
	}
}

func TestContextCommandsRmRejects(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsRmCmd, nil, "index"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsRmCmd, nil, "missing"))
	})
}

// TestContextNewWiki verifies the wiki scaffold: subpath path creation,
// frontmatter contract (kind/id/title/summary/status/created/updated) and the
// default Summary/Claims/Notes body.
func TestContextNewWiki(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC))

	out := string(execute(t, contextNewCmd, nil, "--type", "wiki", "--slug", "backend/auth", "--title", "Auth", "--summary", "auth design"))
	if !strings.Contains(out, filepath.Join(sdtWikiDir, "backend", "auth.md")) {
		t.Fatalf("expected wiki path in output: %q", out)
	}
	page := mustReadFile(t, filepath.Join(sdtWikiDir, "backend", "auth.md"))
	for key, want := range map[string]string{
		"kind":    "wiki",
		"id":      "backend/auth",
		"title":   "Auth",
		"summary": "auth design",
		"status":  "draft",
		"created": "2026-09-20T18:00:00Z",
		"updated": "2026-09-20T18:00:00Z",
	} {
		if got := frontmatterField(page, key); got != want {
			t.Errorf("wiki %s = %q, want %q", key, got, want)
		}
	}
	for _, want := range []string{"## Summary", "## Claims", "## Notes"} {
		if !strings.Contains(page, want) {
			t.Errorf("wiki body missing %q:\n%s", want, page)
		}
	}
}

// TestContextPathWiki verifies subpath support in `context path`.
func TestContextPathWiki(t *testing.T) {
	runInTempDir(t)
	out := string(execute(t, contextPathCmd, nil, "--type", "wiki", "--slug", "backend/session"))
	if !strings.Contains(out, filepath.Join(sdtWikiDir, "backend", "session.md")) {
		t.Errorf("wiki path = %q", out)
	}
}

// buildCommandsStub returns a minimal commands trigger stub for tests.
func buildCommandsStub(id string) string {
	return `---
kind: commands
id: commands/` + id + `
title: ">` + id + ` — agent-invokable task trigger"
summary: "Thin agent command: invoked by the >` + id + ` trigger."
status: active
links:
  - commands/index.md
project: tm
created: "2026-09-20T17:30:00Z"
updated: "2026-09-20T17:30:00Z"
---

<!-- sdt:begin:commands/` + id + ` -->

# ` + "`>" + id + "`" + ` — read this when the command fires

<!-- sdt:end:commands/` + id + ` -->
`
}
