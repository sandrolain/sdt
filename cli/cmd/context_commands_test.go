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

// TestContextCommandsNewPayload verifies the --payload flag end to end: the
// phrase lands in the stub frontmatter (outside the generated markers, so
// `sdt agent init --force` keeps it) and in the commands index Payload
// column, while a trigger created without it renders the explicit
// "not declared" placeholder rather than a blank cell.
func TestContextCommandsNewPayload(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 17, 30, 0, 0, time.UTC))

	execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research", "--payload", "the question the run answers")
	execute(t, contextCommandsNewCmd, nil, "scratch", "--contract", "research")

	stub := mustReadFile(t, "context/commands/triage.md")
	if got := frontmatterField(stub, "payload"); got != "the question the run answers" {
		t.Errorf("stub payload = %q", got)
	}
	// The payload is the index cell: the phrase must appear in the table row.
	idx := mustReadFile(t, "context/commands/index.md")
	if !strings.Contains(idx, "| the question the run answers |") {
		t.Errorf("index Payload cell missing the declared phrase:\n%s", idx)
	}
	if !strings.Contains(idx, commandPayloadUndeclared) {
		t.Errorf("index must show %q for an undeclared trigger:\n%s", commandPayloadUndeclared, idx)
	}

	// Resolution order: a generated trigger keeps the table phrase even when a
	// stale payload field sits in its own frontmatter.
	writeTestFile(t, "context/commands/wiki.md", addCommandPayloadField(buildCommandsStub("wiki"), "stale hand edit"))
	execute(t, contextCommandsNewCmd, nil, "other", "--force", "--contract", "research")
	if got := commandPayloadFor("wiki"); got != declaredCommandPayload("wiki") {
		t.Errorf("generated trigger payload = %q, want the table phrase %q", got, declaredCommandPayload("wiki"))
	}

	// The field survives a body-only regeneration.
	before := mustReadFile(t, "context/commands/triage.md")
	_ = before
	if err := commandsRewriteIndex("tm"); err != nil {
		t.Fatalf("rewrite index: %v", err)
	}
	if got := frontmatterField(mustReadFile(t, "context/commands/triage.md"), "payload"); got != "the question the run answers" {
		t.Errorf("payload lost after index rewrite: %q", got)
	}
}

// TestContextCommandPayloadMissingFile guards the read path: a trigger whose
// file cannot be read resolves to no payload, which the index renders as the
// undeclared placeholder rather than failing the whole regeneration.
func TestContextCommandPayloadMissingFile(t *testing.T) {
	runInTempDir(t)
	if got := contextCommandPayload(filepath.Join(sdtCommandsDir, "absent.md")); got != "" {
		t.Errorf("missing file resolved to %q, want empty", got)
	}
	if got := commandPayloadFor("absent"); got != commandPayloadUndeclared {
		t.Errorf("unreadable trigger = %q, want %q", got, commandPayloadUndeclared)
	}
}

// TestAddCommandPayloadFieldNoFrontmatter guards the defensive path: a rendered
// file without frontmatter is returned untouched rather than having the field
// written outside the frontmatter, where a reader would miss it.
func TestAddCommandPayloadFieldNoFrontmatter(t *testing.T) {
	const bare = "# Title\n\nbody only, no frontmatter\n"
	if got := addCommandPayloadField(bare, "a phrase"); got != bare {
		t.Errorf("frontmatter-less input was modified:\n%s", got)
	}
}

// TestContextCommandsNewPayloadRejects guards the phrase shape: it must be a
// non-empty single line, because it is stored as one frontmatter scalar.
func TestContextCommandsNewPayloadRejects(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--payload", "   "))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--payload", "line one\nline two"))
	})
	if _, err := os.Stat("context/commands/triage.md"); !os.IsNotExist(err) {
		t.Errorf("a rejected --payload must not create the trigger: %v", err)
	}
}

// TestContextCommandsRm verifies the stub deletion and index regeneration.
func TestContextCommandsRm(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/commands/triage.md", addCommandPayloadField(buildCommandsStub("triage"), "the removed trigger's payload"))
	writeTestFile(t, "context/commands/other.md", addCommandPayloadField(buildCommandsStub("other"), "subject of the other command"))

	out := string(execute(t, contextCommandsRmCmd, nil, "triage"))
	if !strings.Contains(out, "removed context/commands/triage.md") {
		t.Errorf("missing removal line in output: %q", out)
	}
	if !strings.Contains(out, "regenerated context/commands/index.md") {
		t.Errorf("missing index regen in output: %q", out)
	}
	if _, err := os.Stat("context/commands/triage.md"); !os.IsNotExist(err) {
		t.Errorf("trigger still present: %v", err)
	}
	if _, err := os.Stat(sdtDeprecatedDir); !os.IsNotExist(err) {
		t.Errorf("commands rm must not write a deprecated copy: %v", err)
	}
	idx := mustReadFile(t, "context/commands/index.md")
	if strings.Contains(idx, ">triage") {
		t.Errorf("index still lists removed trigger:\n%s", idx)
	}
	// A removed trigger's payload column disappears with it; a surviving
	// trigger keeps its own.
	if strings.Contains(idx, "the removed trigger's payload") {
		t.Errorf("index still carries a removed trigger's payload:\n%s", idx)
	}
	if !strings.Contains(idx, "subject of the other command") {
		t.Errorf("index dropped a surviving trigger's payload:\n%s", idx)
	}
	if !strings.Contains(idx, ">other") {
		t.Errorf("index dropped a surviving trigger:\n%s", idx)
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
