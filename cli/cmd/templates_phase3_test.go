package cmd

import (
	"strings"
	"testing"
	"time"
)

// TestCommandsStubTemplateGolden locks the exact bytes of the command trigger
// stub (context commands new / agent init command files).
func TestCommandsStubTemplateGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 17, 30, 0, 0, time.UTC)
	got := instrCommandStubTemplate("triage", "research", "tm", now)
	want := `---
kind: commands
id: commands/triage
title: ">triage — agent-invokable task trigger"
summary: "Thin agent command: invoked by the >triage trigger. Proceeds per context/instructions/research.md (the durable contract). Approve-before-write gate."
status: active # statuses for the commands kind are defined in the matrix (architecture/stack.md)
links:
  - commands/index.md
  - instructions/research.md
sources:
  - instructions/research.md
project: tm
created: "2026-09-20T17:30:00Z"
updated: "2026-09-20T17:30:00Z"
---

# ` + "`>triage`" + ` — read this when the command fires

**Do not re-implement the task here.** The durable contract lives in
` + "`context/instructions/triage.md`" + `; this file is the thin trigger.

## Invocation

- Canonical trigger: ` + "`>triage`" + ` (see ` + "`context/commands/index.md`" + `).
- Scope: ` + "`all`" + ` (default) · single file · glob — see the contract for the
  task's scope semantics.
- No settle: **resolve ` + "`context/commands/triage.md`" + ` → read
  ` + "`context/instructions/research.md`" + ` → proceed per that contract.**

## Gates (non-negotiable)

- **Approve-before-write** — no write before user approval (see the contract).
- Approve-before-archive where the contract moves sources
  (` + "`ingestion/ → refs/`" + `); ` + "`ingestion/`" + `/` + "`refs/`" + ` stay immutable.

## When not to use

- For informational-only questions that need no write: answer inline instead.
- When an active plan already covers the intent: extend that plan instead of
  opening a new task chain.

## Verify

- Per the contract; ` + "`sdt context wiki lint`" + ` + ` + "`sdt context reindex`" + ` clean after writes.
`
	if got != want {
		t.Errorf("stub golden mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestCommandsIndexTemplateGolden locks the exact bytes of the commands index
// (context commands new/rm rewrite and agent init --force regenerate).
func TestCommandsIndexTemplateGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 17, 31, 0, 0, time.UTC)
	got := instrCommandsIndexTemplate([]string{"analysis", "ingestion"}, "tm", now)
	want := `---
kind: commands
id: commands/index
title: "Agent-invokable commands index"
status: active
summary: "Lookup surface for agent-invokable commands: maps each trigger (>ingestion) to its command file under context/commands/ and to its durable instruction under context/instructions/. Durable contracts stay in context/instructions/; command files invoke/reference them, never move or duplicate them."
links:
  - commands/ingestion.md
sources:
  - instructions/ingestion.md
project: tm
created: "2026-09-20T17:31:00Z"
updated: "2026-09-20T17:31:00Z"
---

# Agent-invokable commands index

` + "`context/commands/`" + ` holds one thin file per **agent-visible task**, named by
its trigger. Each command file **invokes or references** its durable contract (a
file under ` + "`context/instructions/`" + `); it never moves or duplicates the contract
text.

Invocation method: an **agent command** — opencode slash-command style,
` + "`>trigger`" + `, e.g. ` + "`>ingestion`" + `. The trigger resolves deterministically to
` + "`context/commands/<trigger>.md`" + `, then to the contract under
` + "`context/instructions/<trigger>.md`" + `.

## Registry

| Trigger | Command file | Durable contract |
|---|---|---|
| ` + "`>analysis`" + ` | ` + "`context/commands/analysis.md`" + ` | ` + "`context/instructions/analysis.md`" + ` |
| ` + "`>ingestion`" + ` | ` + "`context/commands/ingestion.md`" + ` | ` + "`context/instructions/ingestion.md`" + ` |

Lookup order for a trigger ` + "`>T`" + `: ` + "`context/commands/T.md`" + ` → if absent, fall
back to ` + "`context/instructions/T.md`" + `. If neither exists, ask the user (never
invent a contract).

## Invocation contract

- **Approve-before-write:** every execution requires user approval before the
  agent writes anything. No approve gate → no write.
- **Approve-before-archive:** moving sources ` + "`ingestion/ → refs/`" + ` happens only
  with user approval.
- **Sealed sources:** ` + "`ingestion/`" + ` and ` + "`refs/`" + ` are immutable; the only legal
  mutation is the archive move after approval.
- **Scope:** ` + "`all`" + ` (default) · single file · glob — see each command file.

## When not to use

- For informational-only questions that need no write: answer inline instead.
- When an active plan/analysis already covers the intent: integrate the
  existing document instead of starting a new task chain.
`
	if got != want {
		t.Errorf("index golden mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestScriptsIndexTemplateGolden locks the seed of context/scripts/index.md.
func TestScriptsIndexTemplateGolden(t *testing.T) {
	want := `---
kind: scripts
summary: "Index of utility scripts in context/scripts/ (name · purpose · example). Keep the table in sync: add a row for every script, remove it when deleted."
---

# Scripts Index

Available utility scripts under ` + "`context/scripts/`" + `. Read this index to
find a script (name, purpose, example), then execute it on demand. General
guidance on when/how to add scripts: see ` + "`context/instructions/scripts.md`" + `.

## Available scripts

| Script | Purpose | Example |
|--------|---------|---------|
| _none yet_ | — | — |

## How to add a script

1. Write the script in ` + "`context/scripts/`" + ` (descriptive lowercase
   name, no timestamp; self-documenting first lines).
2. Register it here: add a row under Available scripts (name, purpose, example
   invocation).
3. Rename or delete rows when scripts change — the index must never lie.
`
	if got := scriptsIndexTemplate; got != want {
		t.Errorf("scripts-index golden mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestTopicsSeedGolden locks the seed of context/topics.yaml.
func TestTopicsSeedGolden(t *testing.T) {
	want := "# context/topics.yaml — controlled vocabulary for the `topics` frontmatter field.\n" +
		"#\n" +
		"# Canonical topics are kebab-case slugs. Aliases are accepted by `sdt context lint`\n" +
		"# and reported with the canonical form. Keep the list small and meaningful; a\n" +
		"# topic is a subject, not an initiative (that is `objective`).\n" +
		"#\n" +
		"# topics:\n" +
		"#   context-search:\n" +
		"#     - search\n" +
		"#     - retrieval\n" +
		"#   agent-harness:\n" +
		"#     - harness\n" +
		"#   knowledge-management:\n" +
		"#     - knowledge\n" +
		"#   document-management:\n" +
		"#     - docs\n" +
		"#   viewer:\n" +
		"#     - web\n" +
		"#   cli:\n" +
		"#     - commands\n"
	if got := topicsRegisterTemplate; got != want {
		t.Errorf("topics seed mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestWorkReadmeTemplateGolden locks the static shell of context/README.md and
// verifies the registry-driven Commands block is appended exactly once.
func TestWorkReadmeTemplateGolden(t *testing.T) {
	got := sdtWorkReadmeContent()
	if !strings.HasPrefix(got, "# context/ — Working Directory\n") {
		t.Errorf("README must start with the static title:\n%s", got[:80])
	}
	if !strings.Contains(got, "Store durable facts in `decisions/` (decisions) and `architecture/`; the rest of\n") {
		t.Errorf("README missing the closing static prose")
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("README must end with a trailing newline")
	}
	idx := strings.Index(got, "## Commands")
	if idx < 0 {
		t.Fatalf("README missing Commands section:\n%s", got)
	}
	// Exactly the static shell before the placeholder, then the registry block.
	shell := got[:idx]
	section := got[idx:]
	if strings.Count(section, "## Commands") != 1 {
		t.Errorf("Commands section duplicated")
	}
	if section != sdtWorkCommandsSection() {
		t.Errorf("Commands section mismatch\ngot:\n%s\nwant:\n%s", section, sdtWorkCommandsSection())
	}
	if !strings.Contains(section, ctxTypeHelpText(ctxNewTypes())) {
		t.Errorf("Commands section missing new-types help: %q", ctxTypeHelpText(ctxNewTypes()))
	}
	if !strings.Contains(section, ctxTypeHelpText(ctxPathTypes())) {
		t.Errorf("Commands section missing path-types help: %q", ctxTypeHelpText(ctxPathTypes()))
	}
	if !strings.Contains(section, ctxListHelpText()) {
		t.Errorf("Commands section missing list help: %q", ctxListHelpText())
	}
	if !strings.HasSuffix(shell, "history.\n\n") {
		t.Errorf("static shell must end with a blank line before Commands:\n%q", shell[len(shell)-20:])
	}
}
