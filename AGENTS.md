# AGENTS.md — SDT Project

## Project Overview

**SDT (Smart Developer Tools)** is a Go CLI toolset designed for use by AI agents and developers.
It provides deterministic, composable commands for data manipulation, encoding, cryptography, templating,
project knowledge management, and protocol utilities — all with machine-readable output.

Module: `github.com/sandrolain/sdt`
Go version: 1.26.5 (see `.tool-versions`)
Pure-Go: no CGO.

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
gofmt -w ./cli ./main.go
```

---

## Repository Structure

```
cli/
  main.go         — entry point, sets build-time vars (version, commit, date)
  cmd/            — all cobra commands (one file per command group)
  utils/          — shared utility functions (hashing, encoding, JWT, etc.)
main.go           — duplicate entry point at repo root (legacy)
sdt.context/      — agent working directories (plan/, analysis/, worklog/, notes/, tasks/,
                    archive/, architecture/, decisions/, instructions/) + instruction files (+ generated docs/)
docs/             — auto-generated cobra documentation (sdt docs)
test/             — test fixtures
Taskfile.yml      — task definitions (build, test, lint, check)
```

---

## Code Conventions

- **Language**: all code, comments, and documentation in English
- **New Go files**: create as `.txt` first, then rename to `.go` (VS Code creates corrupted files otherwise)
- **Tests**: every new command must have a `_test.go` file; benchmark tests in a separate `_bench_test.go`
- **Coverage**: minimum 80% per package
- **Lint**: no `golangci-lint` issues before committing
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

- `sdt agent init` — non-destructive bootstrap: `.sdt.yaml` (project/group identity), an `AGENTS.md` with a single tagged `instructions` block holding the general agent instructions (5-phase development lifecycle, knowledge tiers, communication, patterns), `sdt.context/` work dirs (plan, worklog, notes, tasks, tmp, architecture, decisions, archive) + `sdt.context/README.md`, and the per-type instruction files under `sdt.context/instructions/` (project, analysis, plan, tasks, adr, architecture, worklog, notes, reference, cli). In a git repository (.git entry in the current directory), `.gitignore` handling is interactive: you are asked whether to ignore `sdt.context/` and which entries — `--gitignore none|tmp|docs|work|context` picks non-interactively (default `work` = `sdt.context/tmp` + `sdt.context/docs`). Entries are written to `.gitignore` in the same directory as `.sdt.yaml`, wrapped in a `# sdt:start` / `# sdt:end` block; parent directories are never resolved. `project`/`group` are prompted interactively when not passed as flags (default: `<dirname>_<short-path-hash>`). `--yes` accepts defaults without prompting; `--force` refreshes generated content and removes obsolete instruction files.

Implementation in `cli/cmd/agent.go` (merge, identity, work dirs) and `cli/cmd/agentinstructions.go` (instruction file templates).

---

## Project Configuration (`.sdt.yaml`)

Commands that are project-scoped (e.g. `sdt context`) read project/group identity from:

1. Explicit `--project` / `--group` flags (highest priority)
2. `.sdt.yaml` file found by walking up from `$CWD` (like `.git`)
3. Error with descriptive message (no implicit fallback)

Example `.sdt.yaml`:

```yaml
project: myapp_7f2b39e1
group: acme-platform
```

Create with: `sdt agent init --project myapp_7f2b39e1 --group acme-platform --yes`, or `sdt config init --project myapp --group platform` (see also `sdt config show`).

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
| `golang.org/x/crypto` | bcrypt |
| `golang.org/x/text` | Unicode text transforms |
| `github.com/JohannesKaufmann/html-to-markdown` | HTML→Markdown (crawldown) |
| `github.com/gocolly/colly` | Web crawling (crawldown) |
| `codeberg.org/readeck/go-readability/v2` | Article extraction (crawldown) |
| `github.com/makiuchi-d/gozxing` | QR code encode/decode (qrcode) |
| `github.com/pquerna/otp` | TOTP/HOTP generation (totp) |
| `github.com/sethvargo/go-password` | Password generation (password) |
| `github.com/segmentio/ksuid`, `github.com/matoous/go-nanoid/v2` | ID generation (uid) |
| `github.com/hashicorp/go-version` | Version comparison (vman) |

<!-- sdt:begin:instructions -->

## Instructions

This project is managed with SDT. Read the relevant instruction file before acting:

- `sdt.context/instructions/project.md` — project identity and configuration
- `sdt.context/instructions/reference.md` — SDT command reference
- `sdt.context/instructions/cli.md` — CLI usage and examples
- `sdt.context/instructions/analysis.md` — analysis documents (structure + initial scan)
- `sdt.context/instructions/plan.md` — plans + the 5-phase development lifecycle
- `sdt.context/instructions/tasks.md` — per-phase task checklists (verify-step)
- `sdt.context/instructions/adr.md` — ADRs (numbered, append-only, sync → architecture)
- `sdt.context/instructions/architecture.md` — living architecture docs (tier: essential)
- `sdt.context/instructions/worklog.md` — work logs (final reports, append-only)
- `sdt.context/instructions/notes.md` — free-form annotations
- `sdt.context/index.md` — generated knowledge index (`sdt context reindex`)
- `sdt.context/docs/README.md` — per-command reference generated by `sdt context docs` (when present)

Work directories live under `sdt.context/` (`plan/`, `analysis/`, `sdt.context/architecture/`,
`sdt.context/decisions/`, worklog/, notes/, tasks/, archive/, tmp/). Never write or execute
temporary files outside the project. Keep all instruction files concise and technical.

### 5-phase development lifecycle

Follow this cycle for any non-trivial task:

1. **Analysis** — perform it; integrate/modify existing analysis files.
2. **Plan** — create from the analysis; integrate/modify as needed.
3. **Tasks** — from the plan create **one checklist file per phase** in
   `sdt.context/tasks/<phase>.md` (`sdt context task`).
4. **Execution** — develop the project; create `sdt.context/architecture/` and `sdt.context/decisions/`
   (ADRs) as needed; **update the tasks and plan files** in place.
5. **Final reports** — append `sdt.context/worklog/` and `notes/` entries.

Before closing a phase run the **verify-step**: completeness, coherence, correctness
(prioritize CRITICAL / WARNING / SUGGESTION, degrade gracefully). Then reindex: run
`sdt context reindex` and `sdt context lint`.

When running the project's build, test and lint commands, discover them from the
project (Taskfile, Makefile, package.json, go.mod or similar); if the project defines
none, record them in `sdt.context/instructions/project.md`.

### Knowledge tiers

`sdt.context/index.md` is the single entry point (generated). At session start read it,
then the **essential** tier first ( `sdt.context/architecture/` + `sdt.context/decisions/` — always
versioned),
then only what is needed of the lower tiers (analysis, plan, notes, tasks). `worklog/` /
`archive/` are history. `summary` frontmatter is mandatory everywhere; `links`
is an optional array validated by lint.

Every work file starts with YAML frontmatter:

```yaml
---
kind: worklog      # plan | worklog | notes | tasks | adr | architecture
summary: <one-line description>   # MANDATORY — the index source
context: what triggered this entry
status: active
created: <ISO 8601>
updated: <ISO 8601>
links:             # optional array
  - decisions/0001-something
project: sdt_44f24890
group: sdt_44f24890
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
workflows, create or update the relevant section in the header of this AGENTS.md
(before the tagged block below) and record the change in
`sdt.context/worklog/`. Keep every section concise and technical.

<!-- sdt:end:instructions -->
