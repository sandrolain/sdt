## Instructions

This project is managed with SDT. This file carries two tagged blocks:
`<!-- sdt:begin:instructions -->` (this one) and the write-once
`<!-- sdt:begin:project -->` block below (project conventions).
Project: p · Group: g

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
| `context/instructions/lessons.md` | Recording lessons, Do-Not-Repeat rules or a decision-log entry |
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
| `context/instructions/development.md` | Writing code: style/architecture agreement, library-first, coding behavior (simplicity, diff-discipline), library docs |
| `context/instructions/git.md` | Committing or branching: the commit gate, Conventional Commits and branch rules |
| `context/instructions/browser.md` | Navigating a web page or verifying a rendered layout |
| `context/instructions/browser-tools.md` | Choosing or using a browser-automation tool |
| `context/instructions/vector.md` | Drawing or reviewing a vector artefact (icon, logo, illustration) |
| `context/instructions/vector-svg.md` | Writing or optimising SVG markup |
| `context/commands/` | Invoking an agent command: `>trigger` (e.g. `>ingestion`) → `context/commands/<trigger>.md` → contract `context/instructions/<trigger>.md` (approve before write) |
| `context/roles/` | Working as one of the SDT roles (read the shared rules + your role profile; `sdt agent roles` manages them) |
| `context/sdtdocs/README.md` | Needing per-command docs (`sdt context docs`, when present) |

Each agent-visible task gets **one file** under `context/commands/` (thin
triggers; the durable contract stays under `context/instructions/`).

Work directories live under `context/` (`plan/`, `analysis/`, `architecture/`,
`decisions/`, `proposals/`, `research/`, `prompts/`, worklog/, notes/, tasks/, commands/,
questions/, archive/, tmp/, `scripts/`, `roles/`). Keep all instruction files concise and technical. Bundled
scripts in `context/scripts/` are listed in
`context/scripts/index.md` and executed on demand, never read into context
(see `instructions/scripts.md`).

### 5-stage development lifecycle

Follow this cycle for any non-trivial task:

1. **Analysis** — perform it; integrate/modify existing analysis files. Map the
   objectives the work must satisfy to the phases/tasks they become.
2. **Plan** — create from the analysis; integrate/modify as needed. Every plan
   ends with a dedicated final **Validation phase** (see
   `context/instructions/plan.md`).
3. **Tasks** — after the plan is explicitly approved, create **one task file per phase** in
	`context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md` (`sdt context task`); do not
	create task files while creating the plan or before that approval. A plan without
	task files has no execution value. Phases are **unbounded in count**,
   **small** and each targets **exactly one deliverable/concern** — split a phase
   further the moment it grows beyond a single agent session (full rules in
   `instructions/plan.md`). Each task file carries a `## Design` study of the
   phase's objectives (see `instructions/tasks.md`).
4. **Execution** — work **one task file at a time**, never from the plan;
   **mark it in progress on take-in**, complete items as they finish, scan
   `context/tasks/` for stale in-progress files before starting; create
   `context/architecture/` and `context/decisions/` (decisions) as
   needed; **update the task and plan files** in place. When a task completes and
   produced tracked changes, **propose a Conventional Commits message and ask
   whether to commit** (see `context/instructions/git.md`) — never commit
   without explicit approval.
5. **Final reports** — append `context/worklog/` and `notes/` entries.

Before closing a phase run the **verify-step**: completeness, coherence, correctness
(prioritize CRITICAL / WARNING / SUGGESTION, degrade gracefully). Then reindex: run
`sdt context reindex` and `sdt context lint`; run `sdt context status` to
detect stale in-progress files; `sdt agent doctor` reports workspace health, and
`sdt agent gate` runs the strict delivery ladder (Build -> Vet -> Lint -> Test,
fail-closed) when a hard gate is wanted. Use a standardized claim vocabulary in task
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

Commits: after each completed task, propose a Conventional Commits message and
**ask before committing** — never commit without explicit user approval (see
`context/instructions/git.md`). Subject ≤50 chars, imperative, lowercase after
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
- **Statuses** — per-type `status` vocabularies (defaults, `draft`/`active`/
  `archived` semantics, `reference` kind) are defined once in the status matrix:
  `context/architecture/stack.md`. Validate before setting.
- **Create with the CLI first** — every `context/` work file is scaffolded
  with `sdt context new --type <t>` (or `sdt context task` for per-phase task
  lists); hand-write only what the CLI does not cover. Timestamps
  (`created`/`updated`) come from `sdt time iso` (RFC3339 UTC). See the
  per-type files for exact commands.

### Keep the chain (recap)

Remember the intent gate: non-trivial intent outside an existing analysis/plan
→ create the **analysis** first. plan → **task files** → **execution** follow
only with **explicit user approval**.
