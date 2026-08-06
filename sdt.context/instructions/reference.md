# SDT — Command Reference

SDT (Smart Developer Tools) is a pure-Go, offline-first CLI for AI agents.
Every command is deterministic and machine-readable.

## Input / Output

- Input: stdin | `--input "string"` | `--file path` | `--inb64`
- Output: `--format text|json|yaml` (default text)
- `--quiet` suppresses informational output; `--no-color` disables ANSI
- Errors: message to stderr + non-zero exit code

## Discover commands

The full, always-current command reference is generated, not written by hand:

```
sdt manifest --format json            # full command tree
sdt schema --command "<command>"      # JSON Schema for one command
sdt context docs                      # per-command docs in sdt.context/docs/
sdt docs                              # full markdown docs per command (humans)
sdt <command> --help                  # usage for a single command
```

## Project configuration

`.sdt.yaml` holds project identity and is found by walking up from the
current directory:

```yaml
project: myapp_7f2b39e1
group: platform
```

Create it with `sdt agent init --project myapp --group platform` or
`sdt config init --project myapp`; inspect with `sdt config show`.

## Memory

`sdt memory` is the persistent, offline key-value store. See
`sdt.context/instructions/memory.md` for full usage.

## CLI usage & examples

The curated command catalog, global flags and practical examples live in
`sdt.context/instructions/cli.md`.
