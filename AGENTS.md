# AGENTS.md — SDT Project

## Project Overview

**SDT (Smart Developer Tools)** is a Go CLI toolset designed for use by AI agents and developers.
It provides deterministic, composable commands for data manipulation, encoding, cryptography, templating,
persistent memory, and protocol utilities — all with machine-readable output.

Module: `github.com/sandrolain/sdt`
Go version: 1.26.2 (see `.tool-versions`)

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
web/              — WASM web application source
web-server/       — static file server for the web interface
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

## Project Configuration (`.sdt.yaml`)

Commands that are project-scoped (e.g. `memory`) read project/group identity from:

1. Explicit `--project` / `--group` flags (highest priority)
2. `.sdt.yaml` file found by walking up from `$CWD` (like `.git`)
3. Error with descriptive message (no implicit fallback)

Example `.sdt.yaml`:

```yaml
project: myapp
group: acme-platform
```

Create with: `sdt memory init --project myapp --group acme-platform`

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
