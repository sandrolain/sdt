package cmd

import (
	"fmt"
	"strings"
	"time"
)

func instrCommandsIndexTemplate(ids []string, project string, now time.Time) string {
	ts := now.UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString(`---
kind: commands
id: commands/index
title: "Agent-invokable commands index"
status: active
summary: "Lookup surface for agent-invokable commands: maps each trigger (>ingestion) to its command file under context/commands/ and to its durable instruction under context/instructions/. Durable contracts stay in context/instructions/; command files invoke/reference them, never move or duplicate them."
links:
  - commands/ingestion.md
sources:
  - instructions/ingestion.md
project: ` + project + `
created: "` + ts + `"
updated: "` + ts + `"
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
`)
	for _, id := range ids {
		fmt.Fprintf(&b, "| `>%s` | `context/commands/%s.md` | `context/instructions/%s.md` |\n", id, id, id)
	}
	b.WriteString(`
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
`)

	b.WriteString(`
## When not to use

- For informational-only questions that need no write: answer inline instead.
- When an active plan/analysis already covers the intent: integrate the
  existing document instead of starting a new task chain.
`)
	return b.String()
}

// instrCommandStubTemplate builds the thin command file for one trigger: it
// resolves the trigger and delegates to the durable contract.

func instrCommandStubTemplate(id, contract, project string, now time.Time) string {
	ts := now.UTC().Format(time.RFC3339)
	return `---
kind: commands
id: commands/` + id + `
title: ">` + id + ` — agent-invokable task trigger"
summary: "Thin agent command: invoked by the >` + id + ` trigger. Proceeds per context/instructions/` + contract + `.md (the durable contract). Approve-before-write gate."
status: active
links:
  - commands/index.md
  - instructions/` + contract + `.md
sources:
  - instructions/` + contract + `.md
project: ` + project + `
created: "` + ts + `"
updated: "` + ts + `"
---

# ` + "`>" + id + "`" + ` — read this when the command fires

**Do not re-implement the task here.** The durable contract lives in
` + "`context/instructions/" + id + ".md`" + `; this file is the thin trigger.

## Invocation

- Canonical trigger: ` + "`>" + id + "`" + ` (see ` + "`context/commands/index.md`" + `).
- Scope: ` + "`all`" + ` (default) · single file · glob — see the contract for the
  task's scope semantics.
- No settle: **resolve ` + "`context/commands/" + id + ".md`" + ` → read
  ` + "`context/instructions/" + contract + ".md`" + ` → proceed per that contract.**

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
}
