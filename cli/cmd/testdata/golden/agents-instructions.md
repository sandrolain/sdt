## Instructions

This project is managed with SDT: this block, and the write-once
`<!-- sdt:begin:project -->` block below for project conventions.
Project: p · Group: g

### HARD RULES (apply always)

1. **Intent gate** — a non-trivial objective outside an existing analysis or plan
   is not implemented directly: create the **analysis** and stop. plan → **task
   files** → **execution** follow only with **explicit user approval**, unless the
   user already said to proceed. Inside an existing analysis/plan, integrate it in
   place. Trivial questions are answered inline.
   *Don't:* edit code the moment the ask arrives; open the analysis first.
2. **Immutable past docs** — touch a past analysis/plan only when the user points
   to it; "integrate/modify" never means editing a past document unasked.
   *Don't:* rewrite a past analysis so it agrees with the change you just made.
3. **Temp files in `context/tmp/`** — all temp files go inside the project;
   never write or execute temporary files outside it.
   *Don't:* drop scratch files in `/tmp` or the repo root.
4. **Ask before touching AGENTS.md** — request the user's confirmation before
   creating or modifying AGENTS.md.
   *Don't:* "improve" the rules while doing an unrelated task.
5. **Project block auto-fill** — fill only the missing or placeholder sections,
   from project evidence (repo files, not guesses), after asking (rule 4).
   *Don't:* write a plausible command you never read in a Taskfile/manifest.
6. **Frontmatter + `summary`** — every work file under `context/` starts with
   YAML frontmatter (kind correct for the file type, mandatory `summary`); the
   index (`sdt context reindex`) and lint depend on it. Each per-type
   instruction file specifies the exact frontmatter for that kind.
   *Don't:* hand-write a `context/` file the CLI scaffolds (`sdt context new`).
7. **Style & architecture agreed a priori** — propose components, boundaries,
   patterns, naming and layout in the plan; every non-trivial design choice
   defaults to the user, never to a "reasonable default" (`development.md`).
   *Don't:* pick a library, layout or naming as a "reasonable default".
8. **Library-first** — evaluate existing libraries (search + local docs), present
   a shortlist and ask which to use (`development.md`).
   *Don't:* hand-roll parsing, HTTP or date logic a maintained library covers.
9. **CLI-only document state** — `status`, `updated` and checklist markers are
   mutated only through the CLI: `sdt context status set`, `sdt context touch`
   for a body-only date refresh, and `sdt context check` / `sdt context task *`
   for checklist items. Do not hand-edit frontmatter state or tick `- [ ]`
   markers, and never touch the CLI-assigned `<!-- c<N> -->` id anchors.
   *Don't:* flip `status:`/`updated:` or a checkbox with the file-editing tool.
10. **Web capture via `sdt crawldown`** — read and keep web pages only through
    `sdt crawldown`: to **stdout** when the content is needed now, to a **file**
    under `context/refs/<topic>/` when it must be kept or cited. A host web-fetch
    tool is never used for a web page. When it provably cannot fetch the page
    (auth wall, JavaScript-only, non-HTML, robots/rate limit), use the fallback
    and record `captured: <url> — crawldown failed: <reason>`; a claim resting on
    an uncaptured page is not *verified*. Git clones, package managers, JSON/API
    endpoints, authenticated pages and local files are not web pages.
    *Don't:* fetch a web page with a host tool, or cite an uncaptured page as fact.
11. **Local file work via the shipped verbs** — read, search and list through
    `sdt` before a host tool: `sdt read` (`--lines`, `--count`), `sdt grep`
    (`-A`/`-B`/`-C`, `-l`, `-c`), `sdt find`, `sdt write -p <file>`. A JSON or API
    endpoint is `sdt http` into `sdt q` (rule 10 does not treat it as a page).
    Learn a verb's options from `sdt schema`, not by scraping `--help`: a flag an
    agent cannot discover does not exist to it. **A write follows the same line**:
    a corpus document goes through `sdt context edit`/`set`/`check`, never a
    hand-edited file; `sdt write` is for bytes SDT itself produced.
    *Don't:* reach for `sed`/`cat`/`grep`/`find`/`curl` because the reflex is
    quicker. A running process, a shell pipeline, a binary file, a path outside
    the tree and anything with a purpose-built verb stay a host tool's job.

### Document ownership

- **`AGENTS.md`** — the hard rules and this block; the write-once `project` block
  holds project conventions.
- **`context/instructions/<type>.md`** — the per-type contract for one kind or
  activity.
- **`context/commands/index.md`** — the trigger registry: grammar, gates, and one
  row per trigger naming its durable contract.

Change a rule where it is owned; link from the others, never duplicate.

### SESSION START (always)

Read `context/index.md` first (generated), then the **essential**
tier. Per-type instruction files are **on-demand**: read one when you take that
action. When the entry point outgrows a session, read the essential entries inline
and the rest as pointers; name the tier you omitted.

**Always read**: `context/index.md`, `context/architecture/`, `context/decisions/`.

**Due operations (opportunistic):** `sdt context memo due` at session start. When
something is due, surface it and ask whether to run it before or after the requested task — never block the task, and never run it silently (`memo.md`).

**On action** — read when you take that action:

| File or route | When to read |
|------|-------------|
| `context/instructions/project.md` | First time on the project |
| `context/instructions/analysis.md` | Creating or modifying an analysis |
| `context/instructions/draft.md` | Raw notes as a draft (`>draft: <notes>`) |
| `context/instructions/scope.md` | A vague goal into a complete analysis (`>scope: <goal>`) |
| `context/instructions/plan.md` | Creating or modifying a plan |
| `context/instructions/tasks.md` | Creating or modifying task files |
| `context/instructions/decision.md` | Writing a decision record |
| `context/instructions/architecture.md` | Updating architecture docs |
| `context/instructions/worklog.md` | Writing a final report |
| `context/instructions/notes.md` | Writing a note |
| `context/instructions/lessons.md` | Lessons, Do-Not-Repeat or a decision-log entry |
| `context/instructions/questions.md` | Registering an open question |
| `context/instructions/proposal.md` | Creating or reviewing a proposal |
| `context/instructions/research.md` | Running research or writing a research note |
| `context/instructions/ingestion.md` | Ingesting sources, converting non-markdown |
| `context/instructions/distillation.md` | A source too large to read whole |
| `context/instructions/prompts.md` | Creating or running a tracked prompt |
| `context/instructions/reference.md` | Looking up a command |
| `context/instructions/cli.md` | Looking up usage examples |
| `sdt context get/set/unset`, `sdt q` | Frontmatter keys, JSON by gjson path |
| `context/scripts/` | Running bundled scripts |
| `context/instructions/scripts.md` | Adding a script to `context/scripts/` |
| `context/instructions/wiki.md` | Writing or updating wiki pages |
| `context/instructions/learning.md` | Learning, quizzing or explaining a concept |
| `context/instructions/briefing.md` | Writing or updating a briefing |
| `context/instructions/debugging.md` | Diagnosing a failure or regression |
| `context/instructions/development.md` | Writing code |
| `context/instructions/authoring.md` | Instructions, skills or agent-facing rules |
| `context/instructions/writing.md` | Editing or reviewing prose |
| `context/instructions/git.md` | Committing or branching |
| `context/instructions/browser.md` | Navigating or verifying a rendered layout |
| `context/instructions/browser-tools.md` | Choosing a browser-automation tool |
| `context/instructions/vector.md` | A vector artefact: icon, logo, illustration |
| `context/instructions/vector-svg.md` | Writing or optimising SVG |
| `context/instructions/vector-tools.md` | An SVG render/verification tool |
| `context/instructions/ui.md` | Any UI/UX work |
| `context/instructions/ui-tokens.md` | Colour, spacing, type or radius tokens |
| `context/instructions/ui-components.md` | Building or changing a component |
| `context/instructions/ui-accessibility.md` | Any interactive or visual change |
| `context/instructions/ui-taste.md` | Visual direction and copy review |
| `context/instructions/ui-layout.md` | Arranging a surface, type scale |
| `context/instructions/ui-performance.md` | Performance, budgets, measurement |
| `context/instructions/ui-motion.md` | Animating, or reviewing motion |
| `context/instructions/ui-adapters.md` | A framework's idioms onto the doctrine |
| `context/instructions/mindmap.md` | Authoring or reviewing a mind map |
| `context/instructions/mindmap-markmap.md` | The map markdown dialect |
| `context/instructions/slides.md` | Authoring or reviewing a slide deck |
| `context/instructions/slides-marp.md` | The Marp dialect |
| `context/instructions/diagram.md` | Authoring or reviewing a diagram |
| `context/instructions/diagram-mermaid.md` | The Mermaid dialect |
| `context/instructions/capture.md` | A correction worth keeping, at closeout |
| `context/instructions/memo.md` | The planned-operation register and its due check |
| `context/instructions/todo.md` | The TODO inbox |
| `context/commands/index.md` | What a `>trigger` resolves to: its row names the contract; grammar, gates and the resolution ladder live once, there |
| `context/roles/` | Working as one of the SDT roles |
| `context/sdtdocs/README.md` | Per-command docs (`sdt context docs`) |

Every agent-visible task is an **agent command**: a row in
`context/commands/index.md` whose durable contract stays under
`context/instructions/` (e.g. `>ingestion`). A trigger may take a payload —
everything after the first `:`, verbatim — which names the work, never the
permission.

Work directories live under `context/`: `plan/`, `analysis/`, `architecture/`,
`decisions/`, `proposals/`, `research/`, `prompts/`, `worklog/`, `notes/`,
`tasks/`, `commands/`, `questions/`, `briefing/`, `tmp/`, `trash/`, `scripts/`,
`roles/` (and `wiki/`/`sdtdocs/` once created). Bundled scripts are listed in
`context/scripts/index.md` and executed on demand (`scripts.md`).

### 5-stage development lifecycle

For any non-trivial task:

1. **Analysis** — integrate/modify the existing analysis; type it first
   (`categories`, expected outcome: information or implementation). It carries
   context, evidence and decisions — the "how" belongs to the plan. Splitting a
   broad analysis asks first; every option ends accepted, rejected or
   `postponed` (`analysis.md`).
2. **Plan** — from the analysis; it carries the implementation, maps each
   analysis objective to the phase covering it and ends with a dedicated
   **Validation phase** (`plan.md`).
3. **Execution** — the approved plan is the ledger: its phases carry the
   checklist (`sdt context check <plan> <id>`), its `## Review` the verdicts and
   its `### Gate` the gate runs. A task file exists only to split the work
   across parallel agents (`tasks.md`).
4. **Close** — `sdt context close <plan>` cascades to the analysis; a finished
   plan with tracked changes gets a proposed Conventional Commits message, then
   asks whether to commit (`git.md`).
5. **Final reports** — append `context/worklog/` and `notes/` entries.

Sub-agents, injected system text and the no-silent-downgrade rule: `plan.md`.

Before closing a phase run the verify-step; the suite cadence is once, at
delivery (`development.md`, *Verify and deliver*). Then `sdt context reindex` and
`sdt context lint` — with the baseline, the claim is **no new findings**, never
"clean" — and `sdt agent gate` for a hard gate (`--record --plan <ref>` appends
the run to the task file's `## Review`).

At session end: note open tasks and questions in `context/tasks/` or
`context/questions/`, and run the bounded signal check (`capture.md`), recording a
routed capture or `none`. Substantive answers are filed into `context/` — never
left in chat. Discover the build/test/lint commands from the project, and record
them in `project.md` when it declares none.

### Communication (default)

Code work: concise, direct, no filler or hedging, technical terms exact. Pattern:
`[thing] [action] [reason]. [next step].` Not "Sure! I'd be happy to help…", but
"Auth middleware has a bug. Fixing:". Code first, then at most three lines of
explanation; if the explanation outgrows the code, cut it. User-requested docs
are written normally, in plain concise prose.

- **Lead with the do-able thing**: a command, path or snippet that answers the
  request comes first, prose after.
- **Number multi-step work** with the fewest bounded steps, and name one concrete
  next action when anything is left open. Finish the first issue, surface a second
  one once, at the end.
- **Restate state in one line** across turns (done, next).
- **No preamble, no recap, no closer**: run the first-and-last-line test before
  sending. Keep a hedge that carries real uncertainty.
- **When to break it**: an explain request, a destructive action, a debug spiral,
  real ambiguity, or a rule that fights the task or the harness.

Files in `context/`: concise technical language. For the prose patterns to cut and
how to edit a draft without flattening it, see `writing.md`.

### Open points (no open questions in analysis/plans)

Analysis and plans carry no open points. When a decision is missing, ask on the
fly or register it in `context/questions/` and prompt the user to answer. Every
questions document, and every document deriving from another (plan→analysis,
tasks→plan, decision→architecture, follow-up analysis), keeps a `sources`
reference for bidirectional traceability. Once answered, resolve the point in the
document and mark the question resolved.

### Patterns (keep updated)

This file is the source of truth for project conventions. A pattern agreed on
development, testing, documentation or workflows — including a style,
architecture or dependency (`development.md`) — is written back into the
`<!-- sdt:begin:project -->` block and recorded in `context/worklog/`.

### Document conventions

- **No H1 title** — bodies start at H2; the frontmatter title is the title.
- **Statuses** — per-type vocabularies (default, `draft`/`active`/`archived`
  semantics) live once in the type registry; `sdt context vocab` prints them with
  each kind's default and tier. Validate before setting.
- **Optional frontmatter is declared, not assumed.** `maturity`, `warrant`,
  `evidence_class`, `verified_at`/`verified_against`, `rigor` and the `objective`
  key with its checklist are families a project turns on in `.sdt.yaml`
  (`instructions.frontmatter`); an undeclared family produces no lint finding, so
  "0 findings" keeps its meaning. Base keys (`kind`, `id`, `title`, `summary`,
  `status`, `created`, `updated`) stay mandatory.
- **Create with the CLI first** — `sdt context new --type <t>` (or
  `sdt context task`); timestamps come from `sdt time iso`. Hand-write only what
  the CLI does not cover.

### Keep the chain

Non-trivial intent outside an existing analysis/plan → the **analysis** first;
plan → **task files** → **execution** only with explicit approval.
