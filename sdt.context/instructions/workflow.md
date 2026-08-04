# Agent Workflow (opinionated)

Follow this loop for any non-trivial task:

1. **Plan** — write a short plan in `sdt.context/plan/` before starting (see planning.md).
2. **Investigate** — read this AGENTS.md, search memory (`sdt memory search`) and inspect the code before changing anything.
3. **Act** — make the smallest change that satisfies the task.
4. **Verify** — run the project's build, test and lint commands.
5. **Annotate** — append a `sdt.context/worklog/` entry describing what changed and why.
6. **Remember** — store durable decisions in `sdt memory`.
7. **Update** — keep the instruction files current when conventions change.

Use `sdt.context/tmp/` for any temporary or scratch files. Never write
or execute temporary files outside the project (e.g. do not use `/tmp`).
