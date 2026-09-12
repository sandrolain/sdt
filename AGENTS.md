# AGENTS.md — SDT Project

<!-- sdt:begin:instructions -->

## Instructions

This project is managed with SDT. This file carries two tagged blocks:
`<!-- sdt:begin:instructions -->` (this one) and the write-once
`<!-- sdt:begin:project -->` block below (project conventions).
Project: sdt_44f24890 · Group: sdt_44f24890

### HARD RULES (apply always)

1. **Intent gate** — a non-trivial user objective/question that falls **outside
   the context of an existing analysis or plan** is not implemented directly:
   create the **analysis** and stop. plan → **task files** → **execution** follow
   only with **explicit user approval**, unless the user already indicated to
   proceed. When the intent **is** inside an existing analysis/plan,
   integrate/modify that document in place. Trivial or informational questions
   are answered inline — the chain starts at the first non-trivial piece of work.
2. **Immutable past docs** — modify past analyses/plans only when the user
   points to it; treat them as immutable otherwise. "integrate/modify" never
   means editing past documents on your own initiative.
3. **Temp files in `context/tmp/`** — all temp files go inside the project;
   never write or execute temporary files outside it.
4. **Ask before touching AGENTS.md** — request the user's confirmation before
   creating or modifying AGENTS.md.
5. **Project block auto-fill** — if the `<!-- sdt:begin:project -->` block is
   missing or has empty/placeholder sections, fill it from project evidence
   (repo files, not guesses); complete only the empty/placeholder parts; ask the
   user first (rule 4 applies).
6. **Frontmatter + `summary`** — every work file under `context/` starts with
   YAML frontmatter (kind correct for the file type, mandatory `summary`); the
   index (`sdt context reindex`) and lint depend on it. Each per-type
   instruction file specifies the exact frontmatter for that kind.

### SESSION START (always)

Read `context/index.md` first (single entry point, generated). Then the
**essential** tier (always versioned). Per-type instruction files are
**on-demand**: read only when you take that action.

**Always read**

- `context/index.md` — generated entry point
- `context/architecture/` — living architecture docs (essential tier)
- `context/decisions/` — ADRs (essential tier)

**On action** — read when you take that action:

| File | When to read |
| ------ | ------------- |
| `context/instructions/project.md` | First time working on the project |
| `context/instructions/analysis.md` | Creating or modifying an analysis |
| `context/instructions/plan.md` | Creating or modifying a plan |
| `context/instructions/tasks.md` | Creating or modifying task files |
| `context/instructions/adr.md` | Writing an ADR |
| `context/instructions/architecture.md` | Updating architecture docs |
| `context/instructions/worklog.md` | Writing a final report |
| `context/instructions/notes.md` | Writing a note |
| `context/instructions/questions.md` | Registering an open question |
| `context/instructions/rfc.md` | Creating or reviewing an RFC |
| `context/instructions/prompts.md` | Creating or running a tracked prompt |
| `context/instructions/reference.md` | Looking up a command |
| `context/instructions/cli.md` | Looking up usage examples |
| `context/scripts/` | Running bundled scripts (see `instructions/scripts.md`) |
| `context/instructions/scripts.md` | Adding or reading scripts in `context/scripts/` |
| `context/instructions/wiki.md` | Writing or updating wiki pages |
| `context/docs/README.md` | Needing per-command docs (`sdt context docs`, when present) |

Work directories live under `context/` (`plan/`, `analysis/`, `architecture/`,
`decisions/`, `rfcs/`, `prompts/`, worklog/, notes/, tasks/,
questions/, archive/, tmp/, `scripts/`). Keep all instruction files concise and technical. Bundled
scripts in `context/scripts/` are listed in
`context/scripts/index.md` and executed on demand, never read into context
(see `instructions/scripts.md`).

### 5-stage development lifecycle

Follow this cycle for any non-trivial task:

1. **Analysis** — perform it; integrate/modify existing analysis files.
2. **Plan** — create from the analysis; integrate/modify as needed.
3. **Tasks** — after the plan is explicitly approved, create **one task file per phase** in
   `context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md` (`sdt context task`); do not
   create task files while creating the plan or before that approval. A plan without
   task files has no execution value. Phases are **unbounded in count**,
   **small** and each targets **exactly one deliverable/concern** — split a phase
   further the moment it grows beyond a single agent session (full rules in
   `instructions/plan.md`).
4. **Execution** — work **one task file at a time**, never from the plan;
   **mark it in progress on take-in**, complete items as they finish, scan
   `context/tasks/` for stale in-progress files before starting; create
   `context/architecture/` and `context/decisions/` (ADRs) as
   needed; **update the task and plan files** in place.
5. **Final reports** — append `context/worklog/` and `notes/` entries.

Before closing a phase run the **verify-step**: completeness, coherence, correctness
(prioritize CRITICAL / WARNING / SUGGESTION, degrade gracefully). Then reindex: run
`sdt context reindex` and `sdt context lint`; run `sdt context status` to
detect stale in-progress files. Use a standardized claim vocabulary in task
files: **passed** (ran and verified), **expected** (written, not run),
**inferred** (static analysis only).

At session end: note open tasks and questions in `context/tasks/` or
`context/questions/` for continuity.

**File answers back**: substantive query answers and discoveries are filed into
`context/` (`notes/`, `analysis/`, `wiki/`) — never left in chat.

When running the project's build, test and lint commands, discover them from the
project (Taskfile, Makefile, package.json, go.mod or similar); if the project defines
none, record them in `context/instructions/project.md`.

### Communication (default)

Code work: terse caveman ultra. Drop articles/filler/pleasantries/hedging.
Fragments OK, short synonyms, technical terms exact, code unchanged.
Pattern: `[thing] [action] [reason]. [next step]`.
Not: "Sure! I'd be happy to help you with that."
Yes: "Bug in auth middleware. Fix:"
Code only — user-requested docs written normal (concise).

Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

Files in `context/`: concise technical language. Cut fluff, keep meaning
and readability. These instructions and docs are concise on purpose.

### Open points (no open questions in analysis/plans)

Analysis and plan documents must NOT contain open points. If a decision is
missing or you are unsure, either:

1. ask on the fly (a short question during the conversation), or
2. register it as an open question in `context/questions/` (one dated
   file with a checklist of open questions) and prompt the user to answer it.

Every questions document, and any document that derives from or extends another
(plan→analysis, tasks→plan, ADR→architecture/analysis, follow-up analysis, ...),
must keep a reference to its source document via the `sources` frontmatter
array and/or inline body text, for bidirectional traceability.

Help the user cover the open points, but keep the user in control of the
decisions. Once answered, resolve the point in the analysis/plan and mark the
question resolved.

### Patterns (keep updated)

This AGENTS.md is the source of truth for project conventions. Whenever a
decision is taken on a pattern to use in development, testing, documentation or
workflows, update the relevant section in the `<!-- sdt:begin:project -->` block
and record the change in
`context/worklog/`. Keep every section concise and technical.

### Keep the chain (recap)

Remember the intent gate: non-trivial intent outside an existing analysis/plan
→ create the **analysis** first. plan → **task files** → **execution** follow
only with **explicit user approval**.

<!-- sdt:end:instructions -->

<!-- sdt:begin:project -->

## Project

### Stack

Go 1.27.1 (see `.tool-versions`) · pure-Go, no CGO · cobra CLI + viper config.

Key dependencies:

- `github.com/spf13/cobra` — CLI framework
- `github.com/spf13/viper` — config file loading
- `github.com/goccy/go-yaml` — YAML marshal/unmarshal
- `github.com/pelletier/go-toml/v2` — TOML support
- `github.com/vmihailenco/msgpack/v5` — MessagePack support
- `github.com/golang-jwt/jwt/v5` — JWT parsing/validation
- `github.com/google/uuid` — UUID v4
- `golang.org/x/crypto` — bcrypt
- `golang.org/x/text` — Unicode text transforms
- `github.com/JohannesKaufmann/html-to-markdown`, `github.com/gocolly/colly`,
  `codeberg.org/readeck/go-readability/v2` — (crawldown)
- `github.com/makiuchi-d/gozxing` — QR code encode/decode (qrcode)
- `github.com/pquerna/otp` — TOTP/HOTP generation (totp)
- `github.com/sethvargo/go-password` — password generation (password)
- `github.com/segmentio/ksuid`, `github.com/matoous/go-nanoid/v2` — ID generation (uid)
- `github.com/hashicorp/go-version` — version comparison (vman)

### Build & Run

```bash
go build -o bin/sdt ./cli
```

Module `github.com/sandrolain/sdt`. Entry `cli/main.go` (sets build-time vars);
`main.go` at repo root is a legacy duplicate.

### Test

```bash
go test ./...
# coverage (minimum 80% required):
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### Lint & Format

```bash
golangci-lint run ./...
govulncheck ./...
gofmt -w ./cli ./main.go
```

### Conventions

- **Language**: all code, comments, and documentation in English.
- **Tests**: every new command must have a `_test.go` file; benchmark tests in a
  separate `_bench_test.go`.
- **Coverage**: minimum 80% per package.
- **Lint**: no `golangci-lint` issues before committing.
- **No CGO**: all dependencies must be pure-Go; no `cgo` usage.
- **Commands**: cobra, one file per command group under `cli/cmd/`. Follow the
  pattern: read input via `getInputString`/`getInputBytes`, read flags via
  `getStringFlag`/`getBoolFlag`/`getIntFlag`, output via `outputString`/`outputBytes`,
  errors via `exitWithError(cmd, err)`. Register with `rootCmd.AddCommand(myCmd)` in `init()`.
- **Global flags** (do not redefine): `--format text|json|yaml`, `--quiet`,
  `--no-color`, `--input`, `--inb64`, `--file`. Read `--format` with `getFormat(cmd)`.
- **Project-scoped commands** (e.g. `sdt context`) read identity from `--project`/`--group`
  flags, then `.sdt.yaml` (walking up from `$CWD` like `.git`); error if absent.
  Create with `sdt agent init --project … --group … --yes` or `sdt config init`.

<!-- sdt:end:project -->
