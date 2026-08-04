# Keeping Instructions Up To Date

AGENTS.md is a thin index. Instruction files under `sdt.context/instructions/`
are the source of truth.

When project conventions, commands or workflows change, update the relevant
instruction file directly, or refresh the generated templates with:

```
sdt agent init --force
```

Record the change in `sdt.context/worklog/`.
