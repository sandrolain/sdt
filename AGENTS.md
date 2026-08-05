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
sdt.context/      — agent working directories + instruction files (see below)
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

## Agent Instruction Files (`sdt agent`)

Agent instructions are managed with `sdt agent init`:

- `sdt agent init` — non-destructive bootstrap: `.sdt.yaml` (project/group identity), an `AGENTS.md` with a single tagged `instructions` block holding the general agent instructions (workflow, planning/work logs/annotations, communication, patterns), `sdt.context/plan|worklog|notes|tmp` + `sdt.context/README.md`, and the CLI-usage instruction files under `sdt.context/instructions/` (project, memory, reference). In a git repository, `.gitignore` is created/updated with an entry ignoring `sdt.context/tmp`. `project`/`group` are prompted interactively when not passed as flags (default: `<dirname>_<short-path-hash>`). `--yes` accepts defaults without prompting; `--force` refreshes generated content and removes obsolete instruction files.

Implementation in `cli/cmd/agent.go` (merge, identity, work dirs) and `cli/cmd/agentinstructions.go` (instruction file templates).

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
- **Phase 3**: tokens, prompt, truncate, schema
- **Phase 4**: cert, hmac, sign/verify, dns, port

<!-- sdt:begin:instructions -->

## Instructions

This project is managed with SDT. Read the relevant instruction file before acting:

- `sdt.context/instructions/project.md` — project identity and configuration
- `sdt.context/instructions/memory.md` — persistent memory usage
- `sdt.context/instructions/reference.md` — SDT command reference

Work directories live under `sdt.context/` (plan/, worklog/, notes/, tmp/).
Never write or execute temporary files outside the project. Keep all instruction
files concise and technical.

### Workflow

Follow this loop for any non-trivial task:

1. **Plan** — write a short plan in `sdt.context/plan/` before starting.
2. **Investigate** — read this AGENTS.md, search memory (`sdt memory search`) and inspect the code before changing anything.
3. **Act** — make the smallest change that satisfies the task.
4. **Verify** — run the project's build, test and lint commands.
5. **Annotate** — append a `sdt.context/worklog/` entry describing what changed and why.
6. **Remember** — store durable decisions in `sdt memory`.
7. **Update** — keep this AGENTS.md current when conventions change.

### Planning, Work Logs & Annotations

Keep planning, work logs and annotations under `sdt.context/`:

```
sdt.context/plan/<YYYY-MM-DD>-<slug>.md              # plan before starting work
sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md      # ordered log of completed work
sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md        # free-form annotations
```

Temporary and scratch files live in `sdt.context/tmp/` — never outside the
project (no `/tmp`, no other absolute paths).

Annotate continuously: plan before starting, append a dated worklog entry after
each change, and record decisions, dead-ends and constraints with
`sdt memory`. The goal is a chronological, searchable history that lets
a future session (agent or human) reconstruct what happened and why.

Every work file starts with YAML frontmatter:

```yaml
---
kind: worklog      # plan | worklog | notes
created_at: <ISO 8601>
context: what triggered this entry
project: <project>
---
```

### Communication (default)

Code work: terse caveman ultra. Drop articles/filler/pleasantries/hedging.
Fragments OK, short synonyms, technical terms exact, code unchanged.
Pattern: `[thing] [action] [reason]. [next step]`.
Not: "Sure! I'd be happy to help you with that."
Yes: "Bug in auth middleware. Fix:"
Code only — user-requested docs written normal (concise).

Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

Files in `sdt.context/`: concise technical language. Cut fluff, keep meaning
and readability. These instructions and docs are concise on purpose.

### Patterns (keep updated)

This AGENTS.md is the source of truth for project conventions. Whenever a
decision is taken on a pattern to use in development, testing, documentation or
workflows, update the relevant section of this AGENTS.md in place and record the
change in `sdt.context/worklog/`. Keep every section concise and technical.

<!-- sdt:end:instructions -->
