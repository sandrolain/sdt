package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func sectionBlock(name, body string) string {
	body = strings.TrimSpace(body)
	return fmt.Sprintf("<!-- sdt:begin:%s -->\n\n%s\n\n<!-- sdt:end:%s -->\n", name, body, name)
}

func sectionBeginMarker(name string) string {
	return "<!-- sdt:begin:" + name + " -->"
}

func sectionEndMarker(name string) string {
	return "<!-- sdt:end:" + name + " -->"
}

func sectionRegexp(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(sectionBeginMarker(name)) + `.*?` + regexp.QuoteMeta(sectionEndMarker(name)) + `\n?`)
}

func hasSection(content, name string) bool {
	return sectionRegexp(name).MatchString(content)
}

// agentMergeBlock ensures the named block exists in content. With force it is
// refreshed, otherwise left untouched when already present. Returns the new
// content and whether it changed.

func agentMergeBlock(content, name, body string, force bool) (string, bool) {
	if hasSection(content, name) {
		if force {
			return sectionRegexp(name).ReplaceAllString(content, "\n"+sectionBlock(name, body)), true
		}
		return content, false
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body), true
}

// agentAppendIfMissing appends the named block only when it is absent. Unlike
// agentMergeBlock it has no force semantics: once present, the block is never
// touched. Used for write-once sections owned by the user/agent.

func agentAppendIfMissing(content, name, body string) string {
	if hasSection(content, name) {
		return content
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body)
}

// ── target file helpers ────────────────────────────────────────────────────────

func agentMergeTarget(target, project, group string, force, wantProject bool) (FileResult, string) {
	res := FileResult{Path: target}
	content := ""
	if _, err := os.Stat(target); err == nil {
		data, rerr := os.ReadFile(target) //#nosec G304 -- user-chosen target file
		if rerr != nil {
			res.Status = statusError
			res.Reason = rerr.Error()
			return res, content
		}
		content = string(data)
	}

	// Instructions block: refreshed with --force, else preserved.
	instrContent, instrChanged := agentMergeBlock(content, agentSectionNameInstructions, agentBlockInstructions(project, group), force)

	// Project block: write-once, created only when absent and requested, never
	// by --force. Declining never removes an already-present block.
	projectContent := instrContent
	projectAdded := false
	if wantProject {
		beforeProject := projectContent
		projectContent = agentAppendIfMissing(projectContent, agentSectionNameProject, agentBlockProject(project, group))
		projectAdded = projectContent != beforeProject
	}

	if !instrChanged && !projectAdded {
		res.Status = statusSkipped
		res.Reason = "AGENTS.md already up to date"
	} else if strings.TrimSpace(content) == "" {
		res.Status = statusCreated
	} else {
		res.Status = statusUpdated
	}
	return res, projectContent
}

func agentBlockInstructions(project, group string) string {
	identity := ""
	if project != "" {
		identity = "Project: " + project
		if group != "" {
			identity += " · Group: " + group
		}
		identity += "\n"
	}

	return `## Instructions

This project is managed with SDT. This file carries two tagged blocks:
` + "`<!-- sdt:begin:instructions -->`" + ` (this one) and the write-once
` + "`<!-- sdt:begin:project -->`" + ` block below (project conventions).
` + identity + `
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
3. **Temp files in ` + "`context/tmp/`" + `** — all temp files go inside the project;
   never write or execute temporary files outside it.
4. **Ask before touching AGENTS.md** — request the user's confirmation before
   creating or modifying AGENTS.md.
5. **Project block auto-fill** — if the ` + "`<!-- sdt:begin:project -->`" + ` block is
   missing or has empty/placeholder sections, fill it from project evidence
   (repo files, not guesses); complete only the empty/placeholder parts; ask the
   user first (rule 4 applies).
6. **Frontmatter + ` + "`summary`" + `** — every work file under ` + "`context/`" + ` starts with
   YAML frontmatter (kind correct for the file type, mandatory ` + "`summary`" + `); the
   index (` + "`sdt context reindex`" + `) and lint depend on it. Each per-type
   instruction file specifies the exact frontmatter for that kind.
7. **Style & architecture agreed a priori** — never invent style or architecture;
   propose both (components, boundaries, patterns, naming, layout) in the plan and
   get explicit user approval before writing code. Every non-trivial design
   choice defaults to the user, not to a "reasonable default" (see
   ` + "`context/instructions/development.md`" + `).
8. **Library-first** — before writing non-trivial code, evaluate existing
   libraries (web search + local docs), present a shortlist and ask the user
   which to use; do not reinvent what a maintained library already provides
   (see ` + "`context/instructions/development.md`" + `).

### SESSION START (always)

Read ` + "`context/index.md`" + ` first (single entry point, generated). Then the
**essential** tier (always versioned). Per-type instruction files are
**on-demand**: read only when you take that action.

**Always read**

- ` + "`context/index.md`" + ` — generated entry point
- ` + "`context/architecture/`" + ` — living architecture docs (essential tier)
- ` + "`context/decisions/`" + ` — decisions (essential tier)

**On action** — read when you take that action:

| File | When to read |
|------|-------------|
| ` + "`context/instructions/project.md`" + ` | First time working on the project |
| ` + "`context/instructions/analysis.md`" + ` | Creating or modifying an analysis |
| ` + "`context/instructions/plan.md`" + ` | Creating or modifying a plan |
| ` + "`context/instructions/tasks.md`" + ` | Creating or modifying task files |
| ` + "`context/instructions/decision.md`" + ` | Writing a decision record |
| ` + "`context/instructions/architecture.md`" + ` | Updating architecture docs |
| ` + "`context/instructions/worklog.md`" + ` | Writing a final report |
| ` + "`context/instructions/notes.md`" + ` | Writing a note |
| ` + "`context/instructions/lessons.md`" + ` | Recording lessons, Do-Not-Repeat rules or a decision-log entry |
| ` + "`context/instructions/questions.md`" + ` | Registering an open question |
| ` + "`context/instructions/proposal.md`" + ` | Creating or reviewing a proposal |
| ` + "`context/instructions/research.md`" + ` | Running or writing a research note |
| ` + "`context/instructions/ingestion.md`" + ` | Ingesting sources; converting non-markdown (anydoc → docling) |
| ` + "`context/instructions/prompts.md`" + ` | Creating or running a tracked prompt |
| ` + "`context/instructions/reference.md`" + ` | Looking up a command |
| ` + "`context/instructions/cli.md`" + ` | Looking up usage examples |
| ` + "`context/scripts/`" + ` | Running bundled scripts (see ` + "`instructions/scripts.md`" + `) |
| ` + "`context/instructions/scripts.md`" + ` | Adding or reading scripts in ` + "`context/scripts/`" + ` |
| ` + "`context/instructions/wiki.md`" + ` | Writing or updating wiki pages |
| ` + "`context/instructions/development.md`" + ` | Writing code: style/architecture agreement, library-first, library docs |
| ` + "`context/commands/`" + ` | Invoking an agent command: ` + "`>trigger`" + ` (e.g. ` + "`>ingestion`" + `) → ` + "`context/commands/<trigger>.md`" + ` → contract ` + "`context/instructions/<trigger>.md`" + ` (approve before write) |
| ` + "`context/sdtdocs/README.md`" + ` | Needing per-command docs (` + "`sdt context docs`" + `, when present) |

Each agent-visible task gets **one file** under ` + "`context/commands/`" + ` (thin
triggers; the durable contract stays under ` + "`context/instructions/`" + `).

Work directories live under ` + "`context/`" + ` (` + "`plan/`" + `, ` + "`analysis/`" + `, ` + "`architecture/`" + `,
` + "`decisions/`" + `, ` + "`proposals/`" + `, ` + "`research/`" + `, ` + "`prompts/`" + `, worklog/, notes/, tasks/, commands/,
questions/, archive/, tmp/, ` + "`scripts/`" + `). Keep all instruction files concise and technical. Bundled
scripts in ` + "`context/scripts/`" + ` are listed in
` + "`context/scripts/index.md`" + ` and executed on demand, never read into context
(see ` + "`instructions/scripts.md`" + `).

### 5-stage development lifecycle

Follow this cycle for any non-trivial task:

1. **Analysis** — perform it; integrate/modify existing analysis files.
2. **Plan** — create from the analysis; integrate/modify as needed.
3. **Tasks** — after the plan is explicitly approved, create **one task file per phase** in
	` + "`context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md`" + ` (` + "`sdt context task`" + `); do not
	create task files while creating the plan or before that approval. A plan without
	task files has no execution value. Phases are **unbounded in count**,
   **small** and each targets **exactly one deliverable/concern** — split a phase
   further the moment it grows beyond a single agent session (full rules in
   ` + "`instructions/plan.md`" + `).
4. **Execution** — work **one task file at a time**, never from the plan;
   **mark it in progress on take-in**, complete items as they finish, scan
   ` + "`context/tasks/`" + ` for stale in-progress files before starting; create
   ` + "`context/architecture/`" + ` and ` + "`context/decisions/`" + ` (decisions) as
   needed; **update the task and plan files** in place.
5. **Final reports** — append ` + "`context/worklog/`" + ` and ` + "`notes/`" + ` entries.

Before closing a phase run the **verify-step**: completeness, coherence, correctness
(prioritize CRITICAL / WARNING / SUGGESTION, degrade gracefully). Then reindex: run
` + "`sdt context reindex`" + ` and ` + "`sdt context lint`" + `; run ` + "`sdt context status`" + ` to
detect stale in-progress files. Use a standardized claim vocabulary in task
files: **passed** (ran and verified), **expected** (written, not run),
**inferred** (static analysis only).

At session end: note open tasks and questions in ` + "`context/tasks/`" + ` or
` + "`context/questions/`" + ` for continuity.

**File answers back**: substantive query answers and discoveries are filed into
` + "`context/`" + ` (` + "`notes/`" + `, ` + "`analysis/`" + `, ` + "`wiki/`" + `) — never left in chat.

When running the project's build, test and lint commands, discover them from the
project (Taskfile, Makefile, package.json, go.mod or similar); if the project defines
none, record them in ` + "`context/instructions/project.md`" + `.

### Communication (default)

Code work: concise, direct. Drop filler/pleasantries/hedging, keep full sentences.
No unnecessary preamble. Technical terms exact, code unchanged.
Pattern: ` + "`[thing] [action] [reason]. [next step]`" + `.
Not: "Sure! I'd be happy to help you with that."
Yes: "Auth middleware has a bug. Fixing:"
Code only — user-requested docs written normal (concise)

Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

Files in ` + "`context/`" + `: concise technical language. Cut fluff, keep meaning
and readability. These instructions and docs are concise on purpose.

### Open points (no open questions in analysis/plans)

Analysis and plan documents must NOT contain open points. If a decision is
missing or you are unsure, either:

1. ask on the fly (a short question during the conversation), or
2. register it as an open question in ` + "`context/questions/`" + ` (one dated
   file with a checklist of open questions) and prompt the user to answer it.

Every questions document, and any document that derives from or extends another
(plan→analysis, tasks→plan, decision→architecture/analysis, follow-up analysis, ...),
must keep a reference to its source document via the ` + "`sources`" + ` frontmatter
array and/or inline body text, for bidirectional traceability.

Help the user cover the open points, but keep the user in control of the
decisions. Once answered, resolve the point in the analysis/plan and mark the
question resolved.

### Patterns (keep updated)

This AGENTS.md is the source of truth for project conventions. Whenever a
decision is taken on a pattern to use in development, testing, documentation or
workflows — including a style, architecture or dependency agreed with the user
(see ` + "`context/instructions/development.md`" + `) — write it back into the relevant
section of the ` + "`<!-- sdt:begin:project -->`" + ` block and record the change in
` + "`context/worklog/`" + `. Keep every section concise and technical.

### Document conventions

- **No H1 title** — document bodies start at H2; the frontmatter title is the
  document title, rendered once by the viewer.

### Keep the chain (recap)

Remember the intent gate: non-trivial intent outside an existing analysis/plan
→ create the **analysis** first. plan → **task files** → **execution** follow
only with **explicit user approval**.
`
}

// agentBlockProject returns the write-once, single generic template placed in
// the `<!-- sdt:begin:project -->` block. The user or agent fills in the
// sections over time; the template does not vary by project type.

func agentBlockProject(project, group string) string {
	return `## Project

<!-- Fill in the sections below. Delete what does not apply. -->

### Stack
<!-- e.g. Go 1.26 · PostgreSQL · gRPC · ... -->

### Build & Run
<!-- e.g. go build -o bin/app ./cmd/app · docker compose up · ... -->

### Test
<!-- e.g. go test ./... · npm test · pytest · ... -->

### Lint & Format
<!-- e.g. golangci-lint run · eslint --fix · ruff format · ... -->

### Conventions
<!-- Project-specific coding conventions, naming rules, branching strategy, ... -->
`
}

// ── registration ───────────────────────────────────────────────────────────────
