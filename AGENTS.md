# AGENTS.md — SDT Project

## Project Overview

**SDT (Smart Developer Tools)** is a Go CLI toolset designed for use by AI agents and developers.
It provides deterministic, composable commands for data manipulation, encoding, cryptography, templating,
persistent memory, and protocol utilities — all with machine-readable output.

Module: `github.com/sandrolain/sdt`
Go version: 1.26.4 (see `.tool-versions`)

---

## Build and Test Commands

```bash
# Build the CLI binary
go build -o bin/sdt ./cli

# Run all tests
go test ./...

# Run tests with coverage (minimum 80% required)
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# Run linter
golangci-lint run ./...

# Run vulnerability check
govulncheck ./...

# Format code
gofmt -w ./cli/...
```

---

## Repository Structure

```
cli/
  main.go         — entry point, sets build-time vars (version, commit, date)
  cmd/            — all cobra commands (one file per command group)
  utils/          — shared utility functions (hashing, encoding, JWT, etc.)
context/
  project.md      — persistent project description
  changes.md      — changelog (append on every change)
  analysis-ai-agent-tooling.md — roadmap document
docs/             — auto-generated cobra documentation (sdt docs)
```

---

## Code Conventions

- **Language**: all code, comments, and documentation in English
- **New Go files**: create as `.txt` first, then rename to `.go` (VS Code creates corrupted files otherwise)
- **Tests**: every new command must have a `_test.go` file; benchmark tests in a separate `_bench_test.go`
- **Coverage**: minimum 80% per package
- **Lint**: no `golangci-lint` issues before committing
- **Commit messages**: Conventional Commits format (`feat:`, `fix:`, `refactor:`, etc.)
- **No CGO**: all dependencies must be pure-Go; no `cgo` usage

---

## Command Authoring Guidelines

Every new command file must follow this pattern:

```go
var myCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "One-line description",
    Long:  `Detailed description`,
    Run: func(cmd *cobra.Command, args []string) {
        // 1. Read input via getInputString / getInputBytes / getInputStringOrFlag
        // 2. Read flags via getStringFlag / getBoolFlag / getIntFlag
        // 3. Process
        // 4. Output via outputString / outputBytes
        // 5. Errors via exitWithError(cmd, err)
    },
}

func init() {
    // add flags
    rootCmd.AddCommand(myCmd)
}
```

Global flags available on all commands (do not redefine):

- `--format text|json|yaml` — output format
- `--quiet` — suppress informational output
- `--no-color` — disable ANSI
- `--input`, `--inb64`, `--file` — input sources

Use `getFormat(cmd)` to read the format flag.

---

## Agent Instruction Tools (`sdt agent`)

`AGENTS.md` files for AI agents are managed with the `sdt agent` group:

- `sdt agent init` — single non-destructive bootstrap: `.sdt.yaml` (project/group identity), opinionated `AGENTS.md` in tagged sections, skill guide in `.agents/skills/sdt/`, and `sdt.context/plan|worklog|notes|tmp` + `sdt.context/README.md`. Idempotent; second run updates without overwriting. `project`/`group` are prompted interactively when not passed as flags (default: `<dirname>_<short-path-hash>`). `--yes` accepts defaults without prompting; `--force` refreshes template sections.
- `sdt agent section list|show|add|update|set|remove` — manage tagged sections delimited by `<!-- sdt:begin:NAME -->` / `<!-- sdt:end:NAME -->` (name: `^[a-z0-9][a-z0-9-]*$`).
- `sdt agent guide` — generate the extended skill guide (SKILL/REFERENCE/WORKFLOWS) with `--dir`, `--force`, `--dry-run`.

Implementation in `cli/cmd/agent.go` (sections/templates) and `cli/cmd/agentguide.go` (guide + shared file writers).

---

## Project Configuration (`.sdt.yaml`)

Commands that are project-scoped (e.g. `memory`) read project/group identity from:

1. Explicit `--project` / `--group` flags (highest priority)
2. `.sdt.yaml` file found by walking up from `$CWD` (like `.git`)
3. Error with descriptive message (no implicit fallback)

Example `.sdt.yaml`:

```yaml
project: myapp_7f2b39e1
group: acme-platform
```

Create with: `sdt agent init --project myapp_7f2b39e1 --group acme-platform --yes`

---

## Persistent Memory (`sdt memory`)

- Storage: `~/.sdt/memory.sqlite` (pure-Go `modernc.org/sqlite`, no CGO)
- Full-text search via SQLite FTS5 (BM25 ranking, unicode61 tokenizer)
- Schema defined in `cli/cmd/memorystore.go`
- No external services required; fully offline

---

## After Every Change

1. Run `go test ./...` — all tests must pass, ≥80% coverage
2. Run `golangci-lint run ./...` — no issues
3. Run `govulncheck ./...` — no vulnerabilities
4. Append a short entry to `context/changes.md` with today's date

---

## Context Directory

The `context/` directory contains persistent notes for agents and developers:

- `context/project.md` — project description and architecture overview
- `context/changes.md` — changelog (append only)
- `context/analysis-ai-agent-tooling.md` — full roadmap document

Do not reference this directory in the code, docs, or user-facing output.

---

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config file loading |
| `github.com/goccy/go-yaml` | YAML marshal/unmarshal |
| `github.com/pelletier/go-toml/v2` | TOML support |
| `github.com/vmihailenco/msgpack/v5` | MessagePack support |
| `github.com/golang-jwt/jwt/v5` | JWT parsing/validation |
| `github.com/google/uuid` | UUID v4 |
| `modernc.org/sqlite` | Pure-Go SQLite (memory storage) |
| `golang.org/x/crypto` | bcrypt |
| `golang.org/x/text` | Unicode text transforms |

---

## Roadmap Summary

- **Phase 1** (complete): cleanup, deprecations, global flags, TTY auto-detect
- **Phase 2** (current): manifest, memory, extract, template, env, diff
- **Phase 3**: tokens, prompt, truncate, schema, skill
- **Phase 4**: cert, hmac, sign/verify, dns, port

<!-- sdt:begin:project -->

## Project

- Project: sdt_44f24890
- Group: sdt_44f24890

This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

Discover all capabilities:
```
sdt manifest --format json
sdt schema --command "<command>"
```
## Project Configuration

SDT project identity lives in `.sdt.yaml`:

```yaml
project: sdt_44f24890
group: sdt_44f24890
```

<!-- sdt:end:project -->

<!-- sdt:begin:commands -->

## Build, Test and Lint

Document the exact commands an agent must run to build, test, lint and format
this project. Keep this section minimal and accurate.

```
# replace with the real commands for this project
make build
make test
make lint
```

Whenever these commands change, update this section (see Self-Update below).

<!-- sdt:end:commands -->

<!-- sdt:begin:workflow -->

## Agent Workflow (opinionated)

Follow this loop for any non-trivial task:

1. **Plan** — write a short plan in `sdt.context/plan/` before starting (see Planning and Work Logs).
2. **Investigate** — read this AGENTS.md, search memory (`sdt memory search`) and inspect the code before changing anything.
3. **Act** — make the smallest change that satisfies the task.
4. **Verify** — run the project's build, test and lint commands.
5. **Annotate** — append a `sdt.context/worklog/` entry describing what changed and why.
6. **Remember** — store durable decisions in `sdt memory`.
7. **Update** — keep this AGENTS.md current when conventions change.

Use `sdt.context/tmp/` for any temporary or scratch files. Never write or
execute temporary files outside the project (e.g. do not use `/tmp`).

<!-- sdt:end:workflow -->

<!-- sdt:begin:memory -->

## Persistent Memory

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

<!-- sdt:end:memory -->

<!-- sdt:begin:planning -->

## Planning and Work Logs

Keep planning, work logs and annotations under the `sdt.context/` directory:

```
sdt.context/plan/<YYYY-MM-DD>-<slug>.md              # plan before starting work
sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md      # ordered log of completed work
sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md        # free-form annotations
```

Temporary and scratch files live in `sdt.context/tmp/` — never outside the
project (no `/tmp`, no other absolute paths).

Date/time prefixes keep files sortable and provide a durable history.

Every file starts with YAML frontmatter carrying metadata:

```yaml
---
kind: worklog      # plan | worklog | notes
created_at: 2026-08-02T10:00:00Z
context: what triggered this entry
project: sdt_44f24890
---
```

<!-- sdt:end:planning -->

<!-- sdt:begin:annotations -->

## Work Annotations

Annotate continuously while working:

- Before starting, create the plan file in `sdt.context/plan/`.
- After each change, append a dated entry to `sdt.context/worklog/`.
- Record decisions, dead-ends and constraints with `sdt memory`.

The goal is a chronological, searchable history that lets a future session
(agent or human) reconstruct what happened and why.

<!-- sdt:end:annotations -->

<!-- sdt:begin:self-update -->

## Keeping AGENTS.md Up To Date

This file is organized in tagged sections:

```
<!-- sdt:begin:NAME -->
...content...
<!-- sdt:end:NAME -->
```

Manage sections with:

```
sdt agent section list                 # show sections
sdt agent section show <name>          # show one section
sdt agent section add <name>           # add a section (stdin/--input/--file)
sdt agent section update <name>        # update a section
sdt agent section set <name>           # add or update
sdt agent section remove <name>        # remove a section
sdt agent init --force                 # refresh generated sections
```

When you change project conventions, commands or workflows, update the relevant
section with `sdt agent section update <name>` and record the change in
`sdt.context/worklog/`.

<!-- sdt:end:self-update -->
