package cmd

const instrNotesTemplate = `# Notes

` + "`context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold free-form annotations that
do not fit plan/web log/task files.

## Structure

Low ceremony: frontmatter header + free-form body.

` + codeFence + `markdown
---
kind: notes
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what triggered this note>"
sources: <optional, relative paths to the origins this note restates or answers>
via: <optional, transformation that produced it (docling, crawl, capture…)>
agent: <agent/tool that wrote the note>   # optional, recommended
role: <role that wrote the note>          # optional (backend, frontend, architect, reviewer, pm)
objective: <optional, ties the note to an initiative>
note_type: <optional, e.g. dead-end>
created: "<ISO 8601>"
project: <project>
---

<free-form content>
` + codeFence + `

## Provenance

- Keep the chain **source → transformation → derived document → task/worklog**:
  anything the note restates, extracts or answers points back at its origin via
  ` + "`sources`" + `. ` + "`sources`" + ` values are relative paths resolved against
  ` + "`context/`" + `; broken references are flagged by ` + "`sdt context lint`" + ` (WARNING).
- ` + "`via`" + ` is documentation-only: name the transformation that produced the
  note when it is not a direct reading (e.g. a crawled page, a chat capture).
- A note that derives from an analysis/plan links both directions — ` + "`sources`" + `
  here, and a reference back from the origin document.
- **Attribute the note**: set ` + "`agent`" + ` (and optionally ` + "`role`" + `) so
  ` + "`sdt context list --agent/--role`" + ` can scope recall; ` + "`sdt context lint`" + `
  raises a SUGGESTION when a notes entry has no ` + "`agent`" + `.

## Dedup before write

- **Search first**: before creating a note, check ` + "`context/notes/`" + ` and the
  generated index for an existing note on the same topic.
- **Reinforce, do not duplicate**: extend the equivalent existing note with a
  dated entry (see ` + "`lessons.md`" + `) or link it ` + "`[[notes/<slug>]]`" + `; only
  create a new file when separation is genuinely clearer.
- ` + "`sdt context lint`" + ` flags exact and near-duplicate notes as a
  **SUGGESTION** (word-set Jaccard) — advisory, never merged automatically:
  resolve it by consolidating or by stating the reason for separation.

## Dead-end notes

Mark a rejected or failed approach with ` + "`note_type: dead-end`" + ` plus an
` + "`objective`" + ` so ` + "`sdt context reindex`" + ` groups it under that objective and
` + "`sdt context status`" + ` counts it. Read an objective's dead ends before
reopening its analysis; do not re-run a rejected approach without new evidence.

## Rules

- **No H1 title** — body starts at H2; see AGENTS.md (document conventions).
- Never store secrets, tokens or credentials in a note; ` + "`sdt context lint --security`" + `
  flags credential literals, prompt-injection phrases and invisible/zero-width
  Unicode (advisory WARNING).
- Anything ephemeral-but-useful that is not a plan, analysis, worklog entry or
  task step.
- Tier: **medium** — see ` + "`architecture/stack.md`" + ` for the tier taxonomy.
`
