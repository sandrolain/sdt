# sdt.context/ — Working Directory

This directory holds the agent's planning, work logs, notes and temporary
files for this project.

## Layout

- `plan/` — plans written before starting non-trivial work
- `worklog/` — chronological log of completed work
- `notes/` — free-form annotations
- `tmp/` — temporary and scratch files (never outside this project)

## Conventions

- Files are prefixed with date/time so they sort naturally and keep history:
  - `sdt.context/plan/<YYYY-MM-DD>-<slug>.md`
  - `sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`
  - `sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`
- Temporary files always go in `sdt.context/tmp/` — never outside the project
  (no `/tmp`, no other absolute paths).
- Every file starts with YAML frontmatter:

```yaml
---
kind: worklog      # plan | worklog | notes
created_at: <ISO 8601>
context: what triggered this entry
project: <project>
---
```

Store durable facts in `sdt memory`, not here.
