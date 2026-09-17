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
7. **Style & architecture agreed a priori** — never invent style or architecture;
   propose both (components, boundaries, patterns, naming, layout) in the plan and
   get explicit user approval before writing code. Every non-trivial design
   choice defaults to the user, not to a "reasonable default" (see
   `context/instructions/development.md`).
8. **Library-first** — before writing non-trivial code, evaluate existing
   libraries (web search + local docs), present a shortlist and ask the user
   which to use; do not reinvent what a maintained library already provides
   (see `context/instructions/development.md`).

### SESSION START (always)

Read `context/index.md` first (single entry point, generated). Then the
**essential** tier (always versioned). Per-type instruction files are
**on-demand**: read only when you take that action.

**Always read**

- `context/index.md` — generated entry point
- `context/architecture/` — living architecture docs (essential tier)
- `context/decisions/` — decisions (essential tier)

**On action** — read when you take that action:

| File | When to read |
|------|-------------|
| `context/instructions/project.md` | First time working on the project |
| `context/instructions/analysis.md` | Creating or modifying an analysis |
| `context/instructions/plan.md` | Creating or modifying a plan |
| `context/instructions/tasks.md` | Creating or modifying task files |
| `context/instructions/decision.md` | Writing a decision record |
| `context/instructions/architecture.md` | Updating architecture docs |
| `context/instructions/worklog.md` | Writing a final report |
| `context/instructions/notes.md` | Writing a note |
| `context/instructions/questions.md` | Registering an open question |
| `context/instructions/proposal.md` | Creating or reviewing a proposal |
| `context/instructions/research.md` | Running or writing a research note |
| `context/instructions/ingestion.md` | Ingesting sources; converting non-markdown (anydoc → docling) |
| `context/instructions/prompts.md` | Creating or running a tracked prompt |
| `context/instructions/reference.md` | Looking up a command |
| `context/instructions/cli.md` | Looking up usage examples |
| `context/scripts/` | Running bundled scripts (see `instructions/scripts.md`) |
| `context/instructions/scripts.md` | Adding or reading scripts in `context/scripts/` |
| `context/instructions/wiki.md` | Writing or updating wiki pages |
| `context/instructions/development.md` | Writing code: style/architecture agreement, library-first, library docs |
| `context/commands/` | Invoking an agent command: `>trigger` (e.g. `>ingestion`) → `context/commands/<trigger>.md` → contract `context/instructions/<trigger>.md` (approve before write) |
| `context/sdtdocs/README.md` | Needing per-command docs (`sdt context docs`, when present) |

Each agent-visible task gets **one file** under `context/commands/` (thin
triggers; the durable contract stays under `context/instructions/`).

Work directories live under `context/` (`plan/`, `analysis/`, `architecture/`,
`decisions/`, `proposals/`, `research/`, `prompts/`, worklog/, notes/, tasks/, commands/,
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
   `context/architecture/` and `context/decisions/` (decisions) as
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

Code work: concise, direct. Drop filler/pleasantries/hedging, keep full sentences.
No unnecessary preamble. Technical terms exact, code unchanged.
Pattern: `[thing] [action] [reason]. [next step]`.
Not: "Sure! I'd be happy to help you with that."
Yes: "Auth middleware has a bug. Fixing:"
Code only — user-requested docs written normal (concise)

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
(plan→analysis, tasks→plan, decision→architecture/analysis, follow-up analysis, ...),
must keep a reference to its source document via the `sources` frontmatter
array and/or inline body text, for bidirectional traceability.

Help the user cover the open points, but keep the user in control of the
decisions. Once answered, resolve the point in the analysis/plan and mark the
question resolved.

### Patterns (keep updated)

This AGENTS.md is the source of truth for project conventions. Whenever a
decision is taken on a pattern to use in development, testing, documentation or
workflows — including a style, architecture or dependency agreed with the user
(see `context/instructions/development.md`) — write it back into the relevant
section of the `<!-- sdt:begin:project -->` block and record the change in
`context/worklog/`. Keep every section concise and technical.

### Document conventions

- **No H1 title** — document bodies start at H2; the frontmatter title is the
  document title, rendered once by the viewer.

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
task build                        # CLI -> bin/sdt
task build-viewer                 # web SPA build + embed + sdtviewer -> bin/sdtviewer
task web-install                  # bun install in web/
```

Equal to `go build -o bin/sdt ./cli` for the CLI and
`bun run build` (in `web/`) → copy `web/dist` → `viewer/dist` → `go build -o
bin/sdtviewer ./viewer` for the standalone viewer (embedded SPA via
`//go:embed all:dist`; `viewer/dist` is gitignored except `.gitkeep`).

Module `github.com/sandrolain/sdt`. Entry `cli/main.go` (sets build-time vars);
`main.go` at repo root is a legacy duplicate. `web/` is Bun + Vite + React TS;
DOM tests use **jsdom** (happy-dom breaks DOMPurify).

### Test

```bash
# Go: context/refs holds an immutable clone with C files / broken Go packages,
# so a bare `go test ./...` fails; exclude it:
go test $(go list -e ./... | grep -v /context/) -coverprofile=coverage.out
go tool cover -func=coverage.out

cd web && bun run test          # vitest (jsdom)
```

`task test` wraps the scoped Go command. Known gap: `cli/utils/converter` and
`cli/utils/crawler` are below the 80% per-package target (pre-existing).

### Lint & Format

```bash
golangci-lint run ./cli/... ./viewer/... ./internal/... .
gofmt -w ./cli ./internal ./viewer ./main.go
govulncheck ./...

cd web && bun run lint && bun run fmt:check    # oxlint + oxfmt
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
