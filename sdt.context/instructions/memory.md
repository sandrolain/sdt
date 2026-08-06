# Persistent Memory

Use `sdt memory` to store facts that must survive across sessions:
decisions, constraints, conventions and known-good choices. Storage is a local
SQLite database (`~/.sdt/memory.sqlite`, FTS5 full-text search); fully
offline, no external services.

```
sdt memory set <key> <value> [--tags a,b]   # remember a fact
sdt memory get <key>                         # retrieve a fact
sdt memory search <query> --format json      # full-text search (BM25)
sdt memory list --format json                # list all entries
sdt memory delete <key>                      # forget a fact
sdt memory export                            # export all entries
sdt memory import --file backup.json         # import entries
sdt memory projects                          # list known projects
sdt memory groups                            # list known groups
```

Identity:
- Entries are scoped by project. Project/group come from `--project` /
  `--group` flags or from `.sdt.yaml` (found walking up from the
  current directory).
- Without an identity the commands fail with a descriptive error; `memory projects` /
  `memory groups` are global and need no project.

Rules:
- Use stable, descriptive keys (e.g. `arch:database`, `decision:auth`).
- Prefer durable project facts over transient data.
- Store work progress in `sdt.context/worklog/`, not in memory.
- Review everything at session start: `sdt memory list --format json`.
