# Persistent Memory

Use `sdt memory` to store facts that must survive across sessions:
decisions, constraints, conventions and known-good choices.

```
sdt memory set <key> <value> [--tags a,b]   # remember a fact
sdt memory get <key>                         # retrieve a fact
sdt memory search <query> --format json      # full-text search (BM25)
sdt memory list --format json                # list all entries
sdt memory delete <key>                      # forget a fact
```

Rules:
- Use stable, descriptive keys (e.g. `arch:database`, `decision:auth`).
- Prefer durable project facts over transient data.
- Store work progress in `sdt.context/worklog/`, not in memory.
- Review everything at session start: `sdt memory list --format json`.
