package cmd

const instrScriptsTemplate = `# Scripts (context/scripts/)

` + "`context/scripts/`" + ` is a **dynamic, user/agent-owned workspace** for
executable utility scripts created during work and kept for reuse. It is not a
library to memorize and not generated content: scripts are dropped here when
they are written, listed in the index, and invoked on demand.

## When to add a script

- A workflow needs a repeatable step that is easier as a script (parse,
  transform, orchestrate, generate) than as typed commands.
- The logic is likely to be useful again later — even outside the current task:
  this is the reuse-forward home for them.
- Do NOT add one-off throwaway commands here; use ` + "`context/tmp/`" + ` for
  experiments and only promote a script here when it earns its place.

## Conventions

- **Naming**: descriptive, lowercase, hyphenated (` + "`split-catalog.sh`" + `,
  ` + "`sync-translations.sh`" + `). No timestamps — scripts are living tools,
  not dated records like plans/worklogs.
- **Multiline scripts**: store in ` + "`context/scripts/`" + ` files; single-purpose
  commands can be documented here for reuse.
- **Self-documenting**: the first lines must explain the script and its usage:
  a purpose line, usage example, and any required inputs/environment.
- **Executable**: set the executable bit when appropriate; run with a shell
  explicit in the shebang.

## Index (mandatory registration)

Every script must be registered in ` + "`context/scripts/index.md`" + ` at creation
time. Registering means adding one row to the ` + "`Available scripts`" + ` table:
name, purpose, example invocation. Never add a script without registering it —
an unregistered script is unknown to every future session.

Index workflow:

1. Write the script in ` + "`context/scripts/`" + `.
2. Add/update its row in ` + "`context/scripts/index.md`" + `.
3. Rename or remove the row when the script is renamed or deleted.

## Execution

- Read ` + "`context/scripts/index.md`" + ` to find a script; execute it on demand —
  do not read the whole scripts into context.
- The AGENTS.md On-action table route is informed by this file: seeing the
  ` + "`context/scripts/`" + ` column, read this instruction file first.
`

const scriptsIndexTemplate = `---
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
