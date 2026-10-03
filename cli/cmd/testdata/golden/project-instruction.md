# Project

- Project: p
- Group: g

This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

## Identity

Project-scoped commands resolve project/group from:

1. `--project` / `--group` flags
2. `.sdt.yaml` found by walking up from the current directory (like `.git`)
3. otherwise a descriptive error (no implicit fallback)

Without explicit flags the default identity is `<dirname>_<short-path-hash>`.

## Environment (.env)

Every command loads a project-root `.env` before its body runs:

1. `.env` is found by walking up from the current directory (like `.sdt.yaml`).
2. Only variables **not already set** in the shell environment are loaded — an
   exported value always wins.
3. A missing `.env` is a no-op; malformed lines are skipped, never fatal.
4. The global `--no-env` flag disables the load for a single invocation.

Use it for project credentials (for example an API key) instead of exporting
them in the shell. Keep `.env` git-ignored; never commit secrets.

## Discovering capabilities

```
sdt manifest --format json
sdt schema --command "<command>"
```

## Project-specific conventions

AGENTS.md carries a write-once `<!-- sdt:begin:project -->` block (Overview,
Architecture / entry points, Stack, Build & Run, Test, Lint & Format,
Conventions). Treat that block as the project's **stable-conventions** record:
fill its empty sections from repository evidence, asking the user first, and
promote a convention there only once it has settled. This instruction file is
the **working companion** — keep concrete commands and conventions here while
they are still forming, then promote the stable ones into the block.
