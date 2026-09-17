package cmd

const instrWorklogTemplate = `# Work Logs

` + "`context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` append one dated entry per
completed change: what changed and why. Append-only history; do not rewrite
existing entries.

## Structure

` + codeFence + `markdown
---
kind: worklog
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what triggered this entry>"
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - plan/<date>-<plan-slug>.md
  - tasks/<date>-<plan-slug>-phase-1.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

## <YYYY-MM-DD HH:MM> — <session title>

- **Actor**: <agent-id / human name>
- **Task ref**: <task file, if applicable>
- **Plan ref**: <plan file, if applicable>
- **Duration**: <approx, optional>

**Summary**

<1-3 sentences: what was actually done this session.>

**Changes made**

- <file/module touched, one line each>

**Decisions made**

- <small local decision>: <why>   (architectural → its own decision record, linked here)

**Deviations from plan/task**

- <deviation, and reason>         (silent deviations are the main source of
                                   plan/reality drift — say so explicitly)

**Blockers encountered**

- <blocker> → <resolved how / still open>

**Next steps**

- <what should happen next, if not already in a task file>
` + codeFence + `

## Section descriptions

- **Actor / Task ref / Plan ref / Duration** — metadata block identifying who
  (or which agent) worked, on what task and plan, for how long.
- **Summary** — 1-3 sentences on what was actually done.
- **Changes made** — one line per file/module touched.
- **Decisions made** — only small, local decisions that don't warrant a full
  decision record; anything architecturally significant becomes a decision record and is linked here.
- **Deviations from plan/task** — say explicitly when what was done differs from
  the task/plan. This is what keeps plan/task files trustworthy.
- **Blockers encountered** — the blocker and how it was resolved (or that it's
  still open, pointing to task status).
- **Next steps** — what should happen next, if not already captured in a task
  file.

## Rules

- **No H1 title** — body starts at H2; see AGENTS.md (document conventions).
- Append-only (nothing is edited retroactively); new entries are new dated
  entries.
- Entry per change (final report phase of the 5-stage cycle).
- Tier: **history** — see ` + "`architecture/stack.md`" + ` for the tier taxonomy.
`
