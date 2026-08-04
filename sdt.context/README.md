# sdt.context/ — Working Directory

This directory holds the agent's planning, work logs, notes, instruction files
and temporary files for this project.

## Layout

- `plan/` — plans written before starting non-trivial work
- `worklog/` — chronological log of completed work
- `notes/` — free-form annotations
- `instructions/` — agent instruction files (referenced by AGENTS.md)
- `tmp/` — temporary and scratch files (never outside this project)

## Conventions

- Files are prefixed with date/time so they sort naturally and keep history:
  - `sdt.context/plan/<YYYY-MM-DD>-<slug>.md`
  - `sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`
  - `sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`
- `sdt.context/` files use concise technical language. Cut fluff,
  keep meaning and readability (token-efficient).
- Every work file starts with YAML frontmatter:

```yaml
---
kind: worklog      # plan | worklog | notes
created_at: <ISO 8601>
context: what triggered this entry
project: <project>
---
```

Store durable facts in `sdt memory`, not here.
