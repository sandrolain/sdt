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
	got := instrCommandStubTemplate("triage", "research", commandKindDocument, "", "the question the run answers",
		[]string{">triage: what does ctxquery guarantee about list and search", ">triage: the topic to look up"}, "tm", now)
	want := `---
kind: commands
id: commands/triage
title: ">triage — agent-invokable task trigger"
summary: "Thin agent command: invoked by the >triage trigger. Proceeds per context/instructions/research.md (the durable contract). Approve-before-write gate."
# statuses for the commands kind are defined in the matrix (architecture/stack.md)
status: active
links:
  - commands/index.md
  - instructions/research.md
sources:
  - instructions/research.md
project: tm
created: "2026-09-20T17:30:00Z"
updated: "2026-09-20T17:30:00Z"
---

## ` + "`>triage`" + ` — read this when the command fires

**Do not re-implement the task here.** The durable contract lives in
` + "`context/instructions/research.md`" + `; this file is the thin trigger.

## Invocation

- Canonical trigger: ` + "`>triage`" + ` (see ` + "`context/commands/index.md`" + `).
- Kind: **document**
- Scope: ` + "`all`" + ` (default) · single file · glob — see the contract for the
  task's scope semantics.
- No settle: **resolve ` + "`context/commands/triage.md`" + ` → read
  ` + "`context/instructions/research.md`" + ` → proceed per that contract.**

## Payload

- **Form:** ` + "`>triage`" + ` for no payload · ` + "`>triage: <payload>`" + ` for one. The
  ` + "`:`" + ` is the delimiter; everything after the **first** colon is the payload,
  verbatim, to the end of the turn (a new line starting with ` + "`>`" + ` ends it). No
  quoting, no escaping, no flags, no ` + "`key=value`" + ` grammar.
- **This trigger accepts:** the question the run answers — see
  ` + "`context/instructions/research.md`" + `.
- **Examples:**
  - ` + "`>triage: what does ctxquery guarantee about list and search`" + `
  - ` + "`>triage: the topic to look up`" + `
- **Empty payload:** apply the declared default; if the trigger has no
  meaningful default, ask **one** question. Never infer the subject silently.
- **Precedence:** explicit payload > working-context resolution > ask the user.
  A payload wins even when it contradicts the working context — state the
  contradiction in one line instead of silently choosing.
- **Unusable payload:** repeat what you understood (trigger + payload, one
  line) and ask **one** question. Never discard a payload, never proceed on a
  partial reading.

## Gates (non-negotiable)

- **Approve-before-write** — no write before user approval (see the contract).
  A payload names the work; it never grants the permission.
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

// TestExecuteTriggerResolves proves the >execute workflow trigger is registered
// with a resolvable contract and the plan subject, and that its resolved
// instruction file exists — the same lookup an agent performs when the trigger
// fires.
func TestExecuteTriggerResolves(t *testing.T) {
	var found *commandStub
	for i := range agentCommandStubs {
		if agentCommandStubs[i].id == "execute" {
			found = &agentCommandStubs[i]
			break
		}
	}
	if found == nil {
		t.Fatal(">execute is not registered in agentCommandStubs")
	}
	if found.kind != commandKindWorkflow {
		t.Errorf(">execute kind = %q, want %q", found.kind, commandKindWorkflow)
	}
	if found.subject != ctxTypePlan {
		t.Errorf(">execute subject = %q, want %q", found.subject, ctxTypePlan)
	}
	if found.contract != ctxTypeTasks {
		t.Errorf(">execute contract = %q, want %q", found.contract, ctxTypeTasks)
	}
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	if !generated[found.contract+sdtMarkdownExt] {
		t.Errorf(">execute contract %q has no generated instruction file", found.contract)
	}
}

// TestCommandsIndexHasResolutionLadder asserts the working-context resolution
// ladder is present in the generated index (the single authoritative place the
// workflow stubs reference) with its four rungs and the ask rule.
func TestCommandsIndexHasResolutionLadder(t *testing.T) {
	idx := instrCommandsIndexTemplate([]commandIndexEntry{
		{Trigger: "plan", Contract: "plan", Kind: commandKindWorkflow, Subject: "analysis", Payload: "the analysis the plan is built from"},
	}, "p", time.Now())
	for _, want := range []string{
		"## Working-context resolution",
		"1. **Explicit**",
		"2. **Active / unique**",
		"3. **Under discussion**",
		"4. **Ask**",
		"tie-break to propose, never to decide",
	} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q", want)
		}
	}
}

// TestCommandsIndexTemplateGolden locks the exact bytes of the commands index
// (context commands new/rm rewrite and agent init --force regenerate).
func TestCommandsIndexTemplateGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 17, 31, 0, 0, time.UTC)
	got := instrCommandsIndexTemplate([]commandIndexEntry{
		{Trigger: "analysis", Contract: "analysis", Kind: commandKindDocument, Payload: "subject or scope of the analysis to create or extend"},
		{Trigger: "ingestion", Contract: "ingestion", Kind: commandKindDocument, Payload: "paths and/or URLs to ingest"},
	}, "tm", now)
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

## Agent-invokable commands index

` + "`context/commands/`" + ` holds one thin file per **agent-visible task**, named by
its trigger. Each command file **invokes or references** its durable contract (a
file under ` + "`context/instructions/`" + `); it never moves or duplicates the contract
text.

Invocation method: an **agent command** — opencode slash-command style,
` + "`>trigger`" + `, e.g. ` + "`>ingestion`" + `. The trigger resolves deterministically to
` + "`context/commands/<trigger>.md`" + `, then to the durable contract named in the
registry below (usually ` + "`context/instructions/<trigger>.md`" + `, but a trigger may
declare a different contract, e.g. the ` + "`sdt research`" + ` family resolves to
` + "`context/instructions/research.md`" + `).

### Grammar

` + "```" + `
>id                        # no payload: the command default, or ask
>id: payload               # canonical: the payload starts after the first colon
>id:payload                # tolerated: whitespace after the colon is trimmed
` + "```" + `

1. **Identifier** — a single kebab-case segment, no colon; resolved with the
   lookup order below.
2. **Payload** — everything after the first colon, verbatim, trimmed at both
   ends, to the end of the turn. Free prose: no quoting, no escaping, no flags.
   A command may recognise a small closed set of keywords inside the prose
   (declared in its own file); anything else is context, not a parameter.
3. **Empty payload** — the command applies its declared default; a command with
   no meaningful default asks one question and never infers silently.
4. **Precedence** — explicit payload > working-context resolution > ask the
   user. The payload wins even when it contradicts the working context.
5. **Unusable payload** — the agent repeats what it understood (trigger +
   payload, one line) and asks one question.
6. **Gates are orthogonal** — a payload is a *what*, not a *permission*: it
   never relaxes the gates below.
7. **Per-command variance is intentional** — the grammar is fixed, the payload
   semantics are declared per command: in the ` + "`Payload`" + ` column below, in the
   ` + "`## Payload`" + ` block of its command file, and in its contract.

The grammar is a contract for the agent, not a parser: ` + "`>trigger`" + ` is typed in
the chat, so the whole line — colon and payload included — reaches the agent
verbatim.

## Registry

| Trigger | Kind | Subject | Command file | Durable contract | Payload |
|---|---|---|---|---|---|
| ` + "`>analysis`" + ` | document | — | ` + "`context/commands/analysis.md`" + ` | ` + "`context/instructions/analysis.md`" + ` | subject or scope of the analysis to create or extend |
| ` + "`>ingestion`" + ` | document | — | ` + "`context/commands/ingestion.md`" + ` | ` + "`context/instructions/ingestion.md`" + ` | paths and/or URLs to ingest |

**Kind** — ` + "`document`" + ` (produces/edits a document of a fixed type) or ` + "`workflow`" + `
(advances the lifecycle of the **Subject** document type). A workflow command
resolves its subject via the ladder in *Working-context resolution* below and
**asks** when it is not unique.

Lookup order for a trigger ` + "`>T`" + `: ` + "`context/commands/T.md`" + ` → the durable contract
named in the registry above (` + "`context/instructions/T.md`" + ` by default). If neither
exists, ask the user (never invent a contract).

## Working-context resolution

A **workflow** command (Kind ` + "`workflow`" + `) operates on a specific subject document
of its declared **Subject** type. It resolves that subject through this fixed
ladder — deterministic where possible, explicit where needed, never a guess:

1. **Explicit** — the subject named in the payload (` + "`>plan analysis/2026…-slug`" + `).
   Rung 1 is the ` + "`>id[: payload]`" + ` grammar; it wins even when it contradicts the
   working context (state the contradiction in one line).
2. **Active / unique** — exactly one candidate of the subject type carries the
   "active" status for that type: ` + "`analysis`" + `/` + "`plan`" + ` → ` + "`status: active`" + `;
   ` + "`proposal`" + ` → ` + "`accepted`" + ` or ` + "`review`" + `; ` + "`tasks`" + ` → ` + "`pending`" + ` or ` + "`in-progress`" + `;
   a plan's task files are the files linked from the plan.
3. **Under discussion** — the subject document the session is currently about
   (just read, created or edited in this conversation).
4. **Ask** — 0 or >1 candidate after rungs 1-3: list them and ask. ` + "`updated`" + `
   recency orders the list as a **hint**; it never picks for you.

A **document** command already declares its type by construction; it follows the
same rules whenever it extends an existing document (e.g. ` + "`>architecture`" + `
updating a living doc, ` + "`>worklog`" + ` closing a phase) and the same "ask, don't
guess" default.

` + "`updated`" + ` recency is a **tie-break to propose, never to decide**: with more than
one candidate the agent shows them (most recent first) and asks. There is no
persisted "current document" pointer — resolution is read-only over frontmatter
` + "`status`" + `/` + "`updated`" + `, the ` + "`links`" + `/` + "`sources`" + ` graph and the session context.

## Invocation contract

- **Approve-before-write:** every execution requires user approval before the
  agent writes anything. No approve gate → no write. A payload names the work;
  it never grants the permission.
- **Approve-before-archive:** moving sources ` + "`ingestion/ → refs/`" + ` happens only
  with user approval.
- **Sealed sources:** ` + "`ingestion/`" + ` and ` + "`refs/`" + ` are immutable; the only legal
  mutation is the archive move after approval.
- **Scope:** ` + "`all`" + ` (default) · single file · glob — see each command file.
- **Precedence:** payload > working-context resolution > ask the user.

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

// TestCategoriesSeedGolden locks the seed of context/categories.yaml.
func TestCategoriesSeedGolden(t *testing.T) {
	want := "# context/categories.yaml — controlled vocabulary for the `categories` frontmatter field.\n" +
		"#\n" +
		"# Canonical categories are kebab-case slugs; a category classifies the kind of\n" +
		"# work a document is (bug, issue, new feature, ...), not its subject (that is\n" +
		"# `topics`). Aliases are accepted by `sdt context lint` and reported with the\n" +
		"# canonical form. Seeded with the agreed set; extend or trim to fit the project.\n" +
		"categories:\n" +
		"  bug:\n" +
		"    - bugfix\n" +
		"  issue: []\n" +
		"  new-feature:\n" +
		"    - feat\n" +
		"  feature-change: []\n" +
		"  refactor: []\n" +
		"  improvement:\n" +
		"    - enhancement\n" +
		"  research: []\n"
	if got := categoriesRegisterTemplate; got != want {
		t.Errorf("categories seed mismatch\ngot:\n%s\nwant:\n%s", got, want)
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
	if !strings.HasSuffix(shell, "`.env` git-ignored.\n\n") {
		t.Errorf("static shell must end with a blank line before Commands:\n%q", shell[len(shell)-20:])
	}
}
