# Project

- Project: sdt_44f24890
- Group: sdt_44f24890

This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

## Identity

Project-scoped commands (e.g. `sdt memory`) resolve project/group from:

1. `--project` / `--group` flags
2. `.sdt.yaml` found by walking up from the current directory (like `.git`)
3. otherwise a descriptive error (no implicit fallback)

Without explicit flags the default identity is `<dirname>_<short-path-hash>`.

## Discovering capabilities

```
sdt manifest --format json
sdt schema --command "<command>"
```