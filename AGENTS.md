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
   *Don't:* edit code the moment the ask arrives; open the analysis first.
2. **Immutable past docs** — modify past analyses/plans only when the user
   points to it; treat them as immutable otherwise. "integrate/modify" never
   means editing past documents on your own initiative.
   *Don't:* rewrite a past analysis so it agrees with the change you just made.
3. **Temp files in `context/tmp/`** — all temp files go inside the project;
   never write or execute temporary files outside it.
   *Don't:* drop scratch files in `/tmp` or the repo root.
4. **Ask before touching AGENTS.md** — request the user's confirmation before
   creating or modifying AGENTS.md.
   *Don't:* "improve" the rules while doing an unrelated task.
5. **Project block auto-fill** — if the `<!-- sdt:begin:project -->` block is
   missing or has empty/placeholder sections, fill it from project evidence
   (repo files, not guesses); complete only the empty/placeholder parts; ask the
   user first (rule 4 applies).
   *Don't:* write a plausible command you never read in a Taskfile/manifest.
6. **Frontmatter + `summary`** — every work file under `context/` starts with
   YAML frontmatter (kind correct for the file type, mandatory `summary`); the
   index (`sdt context reindex`) and lint depend on it. Each per-type
   instruction file specifies the exact frontmatter for that kind.
   *Don't:* hand-write a `context/` file the CLI scaffolds (`sdt context new`).
7. **Style & architecture agreed a priori** — never invent style or architecture;
   propose both (components, boundaries, patterns, naming, layout) in the plan and
   get explicit user approval before writing code. Every non-trivial design
   choice defaults to the user, not to a "reasonable default" (see
   `context/instructions/development.md`).
   *Don't:* pick a library, layout or naming as a "reasonable default".
8. **Library-first** — before writing non-trivial code, evaluate existing
   libraries (web search + local docs), present a shortlist and ask the user
   which to use; do not reinvent what a maintained library already provides
   (see `context/instructions/development.md`).
   *Don't:* hand-roll parsing, HTTP or date logic a maintained library covers.
9. **CLI-only document state** — `status`, `updated` and checklist markers are
   mutated only through the CLI: `sdt context status set`, `sdt context touch`
   for a body-only date refresh, and `sdt context check` / `sdt context task *`
   for checklist items. Do not hand-edit frontmatter state or tick `- [ ]`
   markers, and never touch the CLI-assigned `<!-- c<N> -->` id anchors.
   *Don't:* flip `status:`/`updated:` or a checkbox with the file-editing tool.
10. **Web capture via `sdt crawldown`** — read and keep web pages only through
    `sdt crawldown`: to **stdout** when the content is needed in the conversation,
    to a **file** under `context/refs/<topic>/` when it must be kept or cited. A
    host web-fetch tool is never used for a web page. If `sdt crawldown` provably
    cannot fetch it (auth wall, JavaScript-only page, non-HTML endpoint,
    robots/rate-limit block), use the fallback and **record it in the document**:
    `captured: <url> — crawldown failed: <reason>`. A statement resting on an
    uncaptured page is not *verified*. Boundary: git clones, package managers,
    JSON/API endpoints, authenticated pages and local files are not web pages.
    *Don't:* fetch a web page with a host tool, or cite an uncaptured page as fact.

### Document ownership (who owns which text)

- **`AGENTS.md`** — source of truth: the hard rules and this instruction block;
  the write-once `project` block holds project conventions.
- **`context/instructions/<type>.md`** — the per-type contract (frontmatter,
  sections, lifecycle, procedure) for one document kind or activity.
- **`context/commands/<trigger>.md`** — a thin trigger; the durable contract
  stays in `context/instructions/`.

Change a rule where it is owned and link from the other layers; never duplicate
a contract across them.

### SESSION START (always)

Read `context/index.md` first (single entry point, generated). Then the
**essential** tier (always versioned). Per-type instruction files are
**on-demand**: read only when you take that action.

**Always read**

- `context/index.md` — generated entry point
- `context/architecture/` — living architecture docs (essential tier)
- `context/decisions/` — decisions (essential tier)

**Due operations (opportunistic)** — run `sdt context memo due` at session
start; when it reports something due, surface it and ask whether to run it
before or after the requested task — never block the task and never run it
silently (see `context/instructions/memo.md`).

**On action** — read when you take that action:

| File | When to read |
|------|-------------|
| `context/instructions/project.md` | First time working on the project |
| `context/instructions/analysis.md` | Creating or modifying an analysis |
| `context/instructions/draft.md` | Capturing raw notes as a draft analysis (`>draft: <notes>`) |
| `context/instructions/scope.md` | Eliciting a broad or ambiguous objective into a decision-complete analysis (`>scope: <objective>`) |
| `context/instructions/plan.md` | Creating or modifying a plan |
| `context/instructions/tasks.md` | Creating or modifying task files |
| `context/instructions/decision.md` | Writing a decision record |
| `context/instructions/architecture.md` | Updating architecture docs |
| `context/instructions/worklog.md` | Writing a final report |
| `context/instructions/notes.md` | Writing a note |
| `context/instructions/lessons.md` | Recording lessons, Do-Not-Repeat rules or a decision-log entry |
| `context/instructions/questions.md` | Registering an open question |
| `context/instructions/proposal.md` | Creating or reviewing a proposal |
| `context/instructions/research.md` | Running research (`sdt research`) or writing a research note |
| `context/instructions/ingestion.md` | Ingesting sources; converting non-markdown (`sdt doc2md`, anydoc → docling → markitdown) |
| `context/instructions/distillation.md` | Distilling a source too large to read whole into a load-on-demand reference |
| `context/instructions/prompts.md` | Creating or running a tracked prompt |
| `context/instructions/reference.md` | Looking up a command |
| `context/instructions/cli.md` | Looking up usage examples |
| `sdt context get/set/unset`, `sdt q` | Reading/writing frontmatter keys or querying JSON by gjson path (see `instructions/cli.md`) |
| `context/scripts/` | Running bundled scripts (see `instructions/scripts.md`) |
| `context/instructions/scripts.md` | Adding or reading scripts in `context/scripts/` |
| `context/instructions/wiki.md` | Writing or updating wiki pages |
| `context/instructions/briefing.md` | Writing or updating a briefing (per-subject onboarding) |
| `context/instructions/debugging.md` | Diagnosing an unexpected failure or regression |
| `context/instructions/development.md` | Writing code: style/architecture agreement, library-first, coding behavior (simplicity, diff-discipline), library docs |
| `context/instructions/authoring.md` | Writing or reviewing instructions, skills or agent-facing rules |
| `context/instructions/writing.md` | Editing or reviewing prose: patterns to cut, minimum edit, detect mode |
| `context/instructions/git.md` | Committing or branching: the commit gate, Conventional Commits and branch rules |
| `context/instructions/browser.md` | Navigating a web page or verifying a rendered layout |
| `context/instructions/browser-tools.md` | Choosing or using a browser-automation tool |
| `context/instructions/vector.md` | Drawing or reviewing a vector artefact (icon, logo, illustration) |
| `context/instructions/vector-svg.md` | Writing or optimising SVG markup |
| `context/instructions/vector-tools.md` | Choosing or using an SVG render/verification tool |
| `context/instructions/ui.md` | Any UI/UX work: layout, components, colour, type, motion, copy |
| `context/instructions/ui-tokens.md` | Choosing or changing colour, spacing, type or radius tokens |
| `context/instructions/ui-components.md` | Building or changing a component |
| `context/instructions/ui-accessibility.md` | Any interactive or visual change, plus accessibility review |
| `context/instructions/ui-taste.md` | Layout, visual direction, copy, review |
| `context/instructions/ui-layout.md` | Arranging a surface, spacing and the type scale |
| `context/instructions/ui-performance.md` | Performance, budgets and measurement |
| `context/instructions/ui-motion.md` | Animating anything, or reviewing motion that exists |
| `context/instructions/ui-adapters.md` | Mapping a framework's idioms to the UI doctrine |
| `context/instructions/mindmap.md` | Authoring or reviewing a mind map |
| `context/instructions/mindmap-markmap.md` | Writing the map markdown dialect and its markers |
| `context/instructions/slides.md` | Authoring or reviewing a slide deck |
| `context/instructions/slides-marp.md` | Writing the Marp markdown dialect |
| `context/instructions/diagram.md` | Authoring or reviewing a diagram (type, layout, honesty) |
| `context/instructions/diagram-mermaid.md` | Writing the Mermaid text-diagram dialect |
| `context/instructions/capture.md` | Recognizing a reusable correction or friction at closeout or session end |
| `context/instructions/memo.md` | Planned operations: the `context/memo.yaml` register, its due check and how to act on a due item |
| `context/instructions/todo.md` | Capturing a short idea for the centralized TODO inbox, or listing/closing an item |
| `context/commands/` | Invoking an agent command: `>trigger` or `>trigger: payload` (e.g. `>ingestion: context/refs/example-repo`) → `context/commands/<trigger>.md` → its durable contract (usually `context/instructions/<trigger>.md`; the `sdt research` family resolves to `context/instructions/research.md`). Each trigger is a **document** command (produces/edits a document of a fixed type) or a **workflow** command (`>plan`, `>execute`, `>decision`) that advances the lifecycle of a declared subject and resolves it via the working-context ladder — **ask when it is not unique, never guess**. The payload is verbatim after the first `:`; precedence is payload > working-context resolution > ask; a payload never relaxes a gate (approve before write) |
| `context/roles/` | Working as one of the SDT roles (read the shared rules + your role profile; `sdt agent roles` manages them) |
| `context/sdtdocs/README.md` | Needing per-command docs (`sdt context docs`, when present) |

Each agent-visible task gets **one file** under `context/commands/` (thin
triggers; the durable contract stays under `context/instructions/`). A trigger
may take a payload — everything after the first `:`, verbatim, to the end of
the turn. It names the work; it never grants the permission, so the gates
below still apply.

Work directories live under `context/` (`plan/`, `analysis/`, `architecture/`,
`decisions/`, `proposals/`, `research/`, `prompts/`, worklog/, notes/, tasks/, commands/,
questions/, deprecated/, tmp/, `scripts/`, `roles/`). Keep all instruction files concise and technical. Bundled
scripts in `context/scripts/` are listed in
`context/scripts/index.md` and executed on demand, never read into context
(see `instructions/scripts.md`).

### 5-stage development lifecycle

Follow this cycle for any non-trivial task:

1. **Analysis** — perform it; integrate/modify existing analysis files. **Type it
   first**: record its `categories` (from the controlled register) and state the
   expected outcome — information-only vs implementation — asking one question
   only when it changes the outcome. It carries context, evidence and decisions
   only — the "how" belongs to the plan.
   A too-broad analysis is split into siblings only after **asking the user**
   (R1), one analysis may feed several numbered wave plans (R2), and every
   option ends as accepted, rejected or preserved as a `postponed` analysis
   (R3) — see `context/instructions/analysis.md` and `plan.md`.
2. **Plan** — create from the analysis; integrate/modify as needed. It carries
   the implementation ("how") and maps each analysis objective to the phase that
   covers it. Every plan ends with a dedicated final **Validation phase** (see
   `context/instructions/plan.md`).
3. **Tasks** — after the plan is explicitly approved, create the plan's task
	file(s) in `context/tasks/` (`sdt context task`); do not
	create task files while creating the plan or before that approval. A plan without
	task files has no execution value. The default is **one task file per plan**
	holding one `## Phase <n>` section per phase; split into several files only to
	divide the work by context or across sub-agents. Phases are **unbounded in
	count**, **small** and each targets **exactly one deliverable/concern** — split
	a phase further the moment it grows beyond a single agent session (full rules
	in `instructions/plan.md`). The task file carries a `## Design` study of the
	objectives its phases cover (see `instructions/tasks.md`).
4. **Execution** — work **one task file at a time**, never from the plan;
   **mark it in progress on take-in**, complete items as they finish, scan
   `context/tasks/` for stale in-progress files before starting; create
   `context/architecture/` and `context/decisions/` (decisions) as
   needed; **update the task and plan files** in place. When a task completes and
   produced tracked changes, **propose a Conventional Commits message and ask
   whether to commit** (see `context/instructions/git.md`) — never commit
   without explicit approval.
5. **Final reports** — append `context/worklog/` and `notes/` entries.

**Sub-agents (dispatch discipline).** When a phase is split across sub-agents,
each takes **one atomic, bounded task** and never recursively spawns further
agents; the orchestrator owns the plan, the merge and the closeout. A sub-agent
returns a result, never a plan of its own.

**System-directive text.** Machine-injected or gate text carries a parseable tag
(`[SYSTEM: <source>]`) so the agent recognises its own injected text and never
mistakes it for a user instruction — this closes the injection loop where a
gate/closeout message is re-read as a directive.

**No silent downgrades.** When in doubt between two paths, take the heavier one,
and never weaken an agreed plan mid-task without saying so. An approval covers
the stage actually presented: replying to a plan approves that plan, not a later
or different stage — re-ask when the stage changes (HARD RULE 1).

Before closing a phase run the **verify-step**: completeness, coherence, correctness
(prioritize CRITICAL / WARNING / SUGGESTION, degrade gracefully). Then reindex: run
`sdt context reindex` and `sdt context lint`; run `sdt context resume` to
read the recorded execution state and spot a stale in-progress file (the view is
read-only and states that it reports the recorded paper, not live work);
`sdt agent doctor` reports workspace health, and
`sdt agent gate` runs the strict delivery ladder (Build -> Vet -> Lint -> Test,
fail-closed) when a hard gate is wanted — with `--record --plan <ref>` its run is
appended to the task file's `## Review` block as a `### Gate` record, and a
failing run is recorded with the step it failed at, never as a pass. Regenerate
the per-command docs
(`sdt context docs`) when the command help or the version changed. Use a
standardized claim vocabulary in task files: **passed** (ran and verified),
**expected** (written, not run), **inferred** (static analysis only).

At session end: note open tasks and questions in `context/tasks/` or
`context/questions/` for continuity, and run the bounded signal check
(see `context/instructions/capture.md`), recording a routed capture or `none`.

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

Output size: code first, then at most three lines of explanation. If the
explanation is longer than the code, cut the explanation.

**Lead with the do-able thing.** When a command, path or snippet answers the
request, it comes first; prose follows.

**Number multi-step work** with the fewest bounded steps that work; **name one
concrete next action** when anything is left open; finish the first issue and
surface a discovered second one once, at the end. On work spanning turns,
**restate state** in one line (what is done, what is next). Cap and rank a visible
list; keep the rest, do not drop it.

**No preamble, no recap, no closer.** Before sending, run the first-and-last-line
test: delete an opener that announces what you are about to do and a closer that
recaps, then check that the first and last lines say what to do next and what just
happened. Keep a hedge that carries real uncertainty.

**When to break it:** an explain or walk-through request, a destructive action, a
debug spiral, real ambiguity, a rule that fights the task (the options *are* the
answer), or a rule that fights the harness — the constraint wins, the shape stays.

Commits: after each completed task, propose a Conventional Commits message and
**ask before committing** — never commit without explicit user approval (see
`context/instructions/git.md`). Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

Files in `context/`: concise technical language. Cut fluff, keep meaning
and readability. These instructions and docs are concise on purpose. For the
prose patterns to cut and how to edit a draft without flattening it, see
`context/instructions/writing.md`.

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
  document title.
- **Statuses** — per-type `status` vocabularies (defaults, `draft`/`active`/
  `archived` semantics, `reference` kind) are defined once in the status matrix:
  `context/architecture/stack.md`. Validate before setting.
- **Create with the CLI first** — every `context/` work file is scaffolded
  with `sdt context new --type <t>` (or `sdt context task` for task
  files); hand-write only what the CLI does not cover. Timestamps
  (`created`/`updated`) come from `sdt time iso` (RFC3339 UTC). See the
  per-type files for exact commands.

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
- `golang.org/x/crypto` — bcrypt
- `golang.org/x/text` — Unicode text transforms
- `github.com/JohannesKaufmann/html-to-markdown`, `github.com/gocolly/colly`,
  `codeberg.org/readeck/go-readability/v2` — (crawldown)
- `github.com/makiuchi-d/gozxing` — QR code encode/decode (qrcode)
- `github.com/pquerna/otp` — TOTP/HOTP generation (totp)
- `github.com/sethvargo/go-password` — password generation (password)
- `github.com/segmentio/ksuid`, `github.com/matoous/go-nanoid/v2`, stdlib `uuid` — ID generation (uid)
- `github.com/hashicorp/go-version` — version comparison (vman)
- `github.com/lmittmann/tint` — coloured `log/slog` handler
- `yaml` — YAML frontmatter parsing in the viewer metadata panel (`web/`, lazy-loaded chunk)

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
- **Colours**: `web/src/styles/tokens.css` owns every raw colour and the
  semantic aliases. Outside it, migrate a literal only when it is a semantic
  role (surface, border, text tone, state, focus ring, elevation); leave a
  categorical identity (graph cluster, edge or boundary colour) as a raw value
  with a one-line comment saying why.
  `web/src/styles/tokenMigration.test.ts` guards the elevation role.
- **Tests**: every new command must have a `_test.go` file; benchmark tests in a
  separate `_bench_test.go`.
- **Coverage**: minimum 80% per package.
- **Lint**: no `golangci-lint` issues before committing.
- **No CGO**: all dependencies must be pure-Go; no `cgo` usage.
- **Commands**: cobra, one file per command group under `cli/cmd/`. Follow the
  pattern: read input via `getInputString`/`getInputBytes`, read flags via
  `getStringFlag`/`getBoolFlag`/`getIntFlag`, output via `outputString`/`outputBytes`,
  errors via `exitWithError(cmd, err)`. Register with `rootCmd.AddCommand(myCmd)` in `init()`.
- **Global flags** (do not redefine): `--format text|json|yaml`, `--log-format text|json`, `--quiet`,
  `--no-color`, `--input`, `--inb64`, `--file`. Read `--format` with `getFormat(cmd)`.
- **Project-scoped commands** (e.g. `sdt context`) read identity from `--project`/`--group`
  flags, then `.sdt.yaml` (walking up from `$CWD` like `.git`); error if absent.
  Create with `sdt agent init --project … --group … --yes` or `sdt config init`.
- **Instruction-module portability** — the instructions `sdt agent init` writes
  into *every* bootstrapped project must be portable: the agent must not need, or
  know, this repository's own implementation. This rule is **this repository
  only** (SDT develops the instruction modules and the viewer) and is not shipped
  to bootstrapped projects. A new or edited instruction module is judged by six
  tests — **SUBJECT** (portable doctrine, not the viewer's design), **PRESUPPOSITION**
  (nothing a generic project may lack, with an explicit no-op/degradation rule),
  **LOCATION** (a generated surface, never a repo-local path), **PRESCRIPTION**
  (defers to the project/`development.md`; numbers are overridable defaults, not
  law, and a pinned version is re-verified against the installed tool), **DELIVERY**
  (the delivery surface is named and consistent with `agent init`) and **BOUNDARY**
  (non-overlap with `development.md` and sibling modules stated). The
  viewer-independence invariant — no `viewer`/`sdtviewer` token, word-boundary
  matched — is enforced by `internal/templates/templates_test.go`
  (`TestGeneratedSetIsViewerFree`) over the templates, the AGENTS.md instructions
  block and the roles core layer. A mechanical constraint becomes a gate, not
  prose (`context/instructions/authoring.md`).
- **Workflow self-assessment (per plan)** — at the end of **every executed
  plan**, in its final Validation phase next to the crystallized closeout, the
  agent reviews **its own** run and records it in `context/notes/`. Nothing is
  asked to the user; the agent judges from its own evidence.
  - **Difficulties encountered** — the friction actually observed,
    *operational* (a command failed, a gate looped, context was missing, an
    assumption was wrong) *and* *process-level* (the plan or phase split was
    wrong, an instruction was ambiguous, a late gate caught what the verify-step
    should have).
  - **Optimizations for future workflows** — rules or instructions to change, a
    missing tool or CLI verb, cost/velocity wins (tokens, re-read files,
    avoidable steps).
  - **Routed** — every item routed through the table in
    `context/instructions/capture.md`, one link each; an item already routed
    there is `none` for the session-end signal check. This block never
    redefines what makes a signal generalisable: `capture.md` owns that test.
  - **`nessuna`** — an empty section says `nessuna` explicitly, so "nothing
    found" stays distinguishable from "not reviewed".

  ```bash
  bin/sdt context new --type notes --title "<plan title> — workflow self-assessment" \
    --slug <plan-slug>-workflow-self-assessment --note-type self-assessment \
    --agent opencode --objective <objective> --source plan/<plan-file> \
    --summary "<one line>"
  ```

  One note per plan, `sources` to the plan, and the closeout entry links it, so
  a skipped review is visible. Recall:
  `sdt context list --type notes --where note_type=self-assessment`.

<!-- sdt:end:project -->
