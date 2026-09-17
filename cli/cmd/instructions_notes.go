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
created: "<ISO 8601>"
project: <project>
---

<free-form content>
` + codeFence + `

## Rules

- **No H1 title** — body starts at H2; see AGENTS.md (document conventions).
- Anything ephemeral-but-useful that is not a plan, analysis, worklog entry or
  task step.
- Tier: **medium** — see ` + "`architecture/stack.md`" + ` for the tier taxonomy.
`
