# sdt.context/ — Working Directory

This directory holds the agent's planning, work logs, task lists, notes,
instruction files and temporary files for this project.

## Layout

- `plan/` — plans written before starting non-trivial work
- `worklog/` — chronological log of completed work
- `notes/` — free-form annotations
- `tasks/` — active task list (`TODO.md`)
- `archive/` — completed task lists (history)
- `instructions/` — agent instruction files (referenced by AGENTS.md)
- `tmp/` — temporary and scratch files (never outside this project)

## Conventions

- Files are prefixed with date/time so they sort naturally and keep history:
  - `sdt.context/plan/<YYYY-MM-DD>-<slug>.md`
  - `sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`
  - `sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`
  - `sdt.context/tasks/TODO.md` — active task list
  - `sdt.context/archive/<YYYYMMDD-HHMMSS>-<slug>.md` — archived task lists
- `sdt.context/` files use concise technical language. Cut fluff,
  keep meaning and readability (token-efficient).
- Every work file starts with YAML frontmatter:

```yaml
---
kind: worklog      # plan | worklog | notes | tasks
created_at: <ISO 8601>
context: what triggered this entry
project: <project>
---
```

Store durable facts in `sdt memory`, not here.

## Commands

Create and manage work files with `sdt context`:

- `sdt context new --type plan|worklog|notes --slug <slug> [--input ...]` — create a
  file with the correct name and frontmatter
- `sdt context path --type plan|worklog|notes|tasks|tmp|archive [--slug]` — print a
  path without creating anything
- `sdt context list --type plan|worklog|notes|tasks|archive` — list existing files
- `sdt context docs [--clean]` — generate per-command agent docs in `sdt.context/docs/` (gitignored)
- `sdt context task add "<step>"` / `done|block|wip <id>` / `list` / `archive` — manage the
  active task list
