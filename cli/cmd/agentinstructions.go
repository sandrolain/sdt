package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// FileResult is the outcome of creating or updating one file.
type FileResult struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

func outputFileResults(cmd *cobra.Command, results []FileResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(results, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(results)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, r := range results {
			line := fmt.Sprintf("[%s] %s", r.Status, r.Path)
			if r.Reason != "" {
				line += "  # " + r.Reason
			}
			outputString(cmd, line+"\n")
		}
	}
}

// ── instruction file templates ─────────────────────────────────────────────────

func instrProjectTemplate(project, group string) string {
	var b strings.Builder
	b.WriteString(`# Project
`)
	if project != "" {
		fmt.Fprintf(&b, "\n- Project: %s\n", project)
	}
	if group != "" {
		fmt.Fprintf(&b, "- Group: %s\n", group)
	}
	b.WriteString(`
This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

## Identity

Project-scoped commands resolve project/group from:

1. ` + "`--project`" + ` / ` + "`--group`" + ` flags
2. ` + "`.sdt.yaml`" + ` found by walking up from the current directory (like ` + "`.git`" + `)
3. otherwise a descriptive error (no implicit fallback)

Without explicit flags the default identity is ` + "`<dirname>_<short-path-hash>`" + `.

## Discovering capabilities

` + codeFence + `
sdt manifest --format json
sdt schema --command "<command>"
` + codeFence + `

## Project-specific conventions

AGENTS.md carries a write-once ` + "`<!-- sdt:begin:project -->`" + ` block with a
single generic template (Stack, Build & Run, Test, Lint & Format, Conventions).
This file is the companion: record the concrete build/test/lint commands and
project conventions here, and keep the AGENTS.md project block in sync when a
pattern becomes a stable convention.
`)
	return b.String()
}

const instrAnalysisTemplate = `# Analysis Documents

` + "`context/analysis/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold investigation and
implementation plans. They are important while open, less once implemented.

## Purpose

- Record a reasoning/decision process: what was examined, what was decided, why.
- Every analysis has a clear objective that triggered it.

## When to start here (intent first)

A user objective/question that falls **outside the context of an existing
analysis or plan** starts here: create the **analysis** and stop. Each next
step — **plan** → **task files** → **execution** — is taken only with
**explicit user approval**, unless the user has indicated to proceed. When the
intent is in the context of an existing analysis, integrate/modify it in place
(` + "`sources`" + ` back to it); never modify a previous analysis on your own —
only when the user points to it. Trivial or informational questions are
answered inline — the chain starts at the first non-trivial piece of work.

## Structure

` + codeFence + `markdown
---
kind: analysis
title: "<one-line title>"
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<objectives / what triggered the analysis>"
status: active            # active | draft | archived
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:                    # optional related documents
  - decisions/0002-api-format
sources:                  # optional: analysis/plan this derives from or extends
  - analysis/<date>-<slug>.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

# <Title>

## Problem statement
## Current state
## Assumptions & Unknowns
## Non-goals
## Investigation
## Options considered
### Option A: <name>
### Option B: <name>
## Evidence / sources
## Risks and unknowns
## Open questions
## Recommendation
## Next steps
` + codeFence + `

## Section descriptions

- **Problem statement** — what triggered this analysis: a bug, a decision, a
  design question. Be concrete: symptoms observed, not just "investigate X".
- **Current state** — facts as they are today, no opinions. Verifiable by anyone
  re-reading later without re-doing the investigation: code paths, data, logs,
  error messages, versions.
- **Assumptions & Unknowns** — mandatory, do not skip. Anything taken for
  granted goes here with a confidence level (high/medium/low) and the evidence.
  **Rule**: an assumption with confidence ` + "`low`" + ` that materially affects the
  recommendation must be surfaced as an open question, not folded silently into
  the conclusion.
- **Non-goals** — what this analysis deliberately does not cover, to keep it
  bounded.
- **Investigation** — what was tried, checked, tested. Include dead ends; they
  save the next person (human or agent) from repeating them. Format:
  ` + "`<step taken> → <result / finding>`" + `, incl. "ruled out because ...".
- **Options considered** — only if the analysis leads to a choice; otherwise
  omit. One subsection per option: Description, Pros, Cons, Effort/risk.
- **Evidence / sources** — links, benchmark numbers, upstream issues, docs that
  support the findings. Prefer primary sources.
- **Risks and unknowns** — what remains uncertain even after this analysis.
  Distinguish from open questions: risks nobody can answer yet (e.g. depends on
  production data we don't have); open questions a specific person must decide.
- **Open questions** — points requiring a human decision before proceeding. If
  more than 2-3 or they need tracking over time, move them to a dedicated
  questions doc and link via ` + "`sources`" + ` instead of duplicating here.
- **Recommendation** — the conclusion, stated plainly, with confidence level
  (high/medium/low) and why. This is what a plan will be built on top of.
- **Next steps** — checkboxes: e.g. turn this into a plan for the fix, get open
  questions answered before planning.

## Rules

- ` + "`summary`" + ` is mandatory and used by ` + "`sdt context reindex`" + `.
- Dated files: integrate/modify the current analysis while it is the active one;
  a materially new line of investigation gets a new dated file (and sets
  ` + "`sources`" + ` back to the analysis it extends).
- Leave **no open points**: if a decision is missing, ask on the fly or register
  it as an open question in ` + "`context/questions/`" + `; keep the user in
  control of the decisions.
- Track work via the 5-stage cycle: Analysis → Plan → Tasks per phase → Execution
  (updates plan+task, creates architecture/decisions) → Final reports.
- Verify-step before finishing: completeness, coherence, correctness; prioritize
  CRITICAL / WARNING / SUGGESTION and degrade gracefully.

## Initial scan (bootstrap)

On an established (brownfield) or new (greenfield) project, do an initial scan
before changing anything: read the README, docs, code structure and ` + "`git log`" + `.
Offer what to capture and ask confirmation before writing knowledge files
(non auto-capturing). Populate ` + "`architecture/`" + ` and new decisions in
` + "`decisions/`" + `; ` + "`worklog/`" + `/` + "`archive/`" + ` stay history.
`

const instrPlanTemplate = `# Plan Documents

` + "`context/plan/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold the plan for a piece of
work, derived from the related analysis. Plans are **living documents**: they are
updated during execution, not frozen. A plan exists to produce task files; a plan
without tasks has no execution value.

## Intent first

A plan is **required before implementation** and **derives from an analysis**.
For a new user objective/question outside the context of an existing analysis
or plan, the order is: analysis → (approval) → plan → (approval) → task files →
(approval) → implementation — each step gated by **explicit user approval**
unless the user indicated to proceed. The plan is built on top of the analysis
(set ` + "`sources`" + `). Within a plan that is already running,
integrate/modify it in place — do not restart the chain; never modify a
previous analysis/plan on your own.

## Workflow (plan → tasks → execution)

Follow strictly — the plan defines the work, the task files execute it:

1. **Create the plan** — write Objective, Constraints and assumptions, Out of
   scope, Phases, Verification and Completion criteria from the analysis.
2. **Wait for plan approval** — stop after creating the plan and wait for
  explicit user approval before creating task files. Do not create task files
  as part of plan creation.
3. **Create task files after approval** — create **one task file per phase**
  (` + "`context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md`" + `, see
  ` + "`instructions/tasks.md`" + `). Every phase maps to a task file; link in both
  directions (plan frontmatter → task files, task file frontmatter → plan).
4. **Wait for task approval** — stop after creating the task files and wait for
  explicit user approval before implementation.
5. **Execute one task file at a time** — never work from the plan itself. Pick
   a task file with no pending dependencies, mark it **in progress** on
   take-in, complete items as they finish, then move to the next.
6. **Update in place** — plan and task files are **living**: reflect progress,
   decisions and deviations as they happen; keep ` + "`updated`" + ` current.

## Structure

` + codeFence + `markdown
---
kind: plan
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<objective / what triggered the plan>"
status: active
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - analysis/20260905-...-analysis-slug.md
  - tasks/<date>-<plan-slug>-phase-1.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

# <Title>

## Objective
## Constraints and assumptions
## Out of scope
## Phases (each maps to a task file)
## Dependency graph
## Completion criteria
## Review log
` + codeFence + `

## Section descriptions

- **Objective** — 1-3 sentences: what this plan achieves and why. Must be
  clear enough that someone reading only this section understands the goal.
- **Constraints and assumptions** — explicit limitations and assumptions. If an
  assumption is a guess (low confidence) and getting it wrong would be costly,
  resolve it before marking the plan approved instead of building tasks on it.
- **Out of scope** — work explicitly excluded, to prevent scope creep. Be
  specific: not just "other things" but named areas.
- **Phases** — each phase maps to one task file. Checklists here are at TASK
  granularity (one line per task); step-level detail lives in the task file.
  Each phase has: **Goal** (one sentence), **Depends on** (none / Phase X), and
  a task list. **Phase rules** (user-mandated):
  1. one phase = **one deliverable/concern**, stated in its Goal sentence;
  2. **no cap on the number of phases** — a plan may have any count;
  3. keep phases **small**: aim for ~5-7 checklist items; a >10-item checklist
     triggers a ` + "`sdt context lint`" + ` SUGGESTION to split the phase;
  4. a phase must be completable by a **single agent in one focused session**
     without re-reading the whole plan;
  5. when refining an existing phase, use alphanumeric sub-phase suffixes
     (` + "`1a`" + `, ` + "`1b`" + `) instead of widening it.
- **Good vs bad splitting** — bad: one "Implement feature" phase with 15 mixed
  items (schema + backend + CLI + tests in one file). Good: split into phases,
  each with a single deliverable and ≤ ~5 items: schema → backend → CLI → tests.
- **Dependency graph** — only needed when there are more than ~3 tasks or
  non-obvious ordering. Use a simple ASCII diagram.
- **Completion criteria** — checkboxes for global exit conditions (all phases
  completed, full test suite green, etc.).
- **Review log** — only if the plan is revised after approval. Not for execution
  progress (that lives in task files). Format: ` + "`- <YYYY-MM-DD>: <what changed and why>`" + `.

## Frontmatter properties

| Property | Mandatory | Description |
|----------|-----------|-------------|
| kind | yes | Document type, fixed value ` + "`plan`" + `; index and lint depend on it. |
| summary | yes | 1-2 sentence summary → index source. |
| context | yes | Objective / what triggered the plan. |
| status | yes | ` + "`active`" + ` / ` + "`completed`" + ` / ` + "`abandoned`" + `. |
| created | yes | ISO 8601 creation date. |
| updated | yes | ISO 8601 last-edit date; refresh on every change. |
| links | yes | Relative paths to source docs (analysis) and task files. |
| project | yes | Project id from ` + "`.sdt.yaml`" + ` (e.g. ` + "`sdt_44f24890`" + `). |
| agent | no | Agent/tool (AI or human) that created or last edited this file. |
| model | no | Model id of that agent (e.g. ` + "`opencode/big-pickle`" + `) — provenance. |
| session | no | Session id for traceability across edits. |

## 5-stage development lifecycle

Every non-trivial piece of work follows this cycle; a plan is phase 2:

1. **Analysis** — perform and integrate/modify the analysis.
2. **Plan** — created from the analysis; integrated/modified as needed.
3. **Tasks** — from the plan create **one task file per phase** (see
   ` + "`instructions/tasks.md`" + `) so the agent can track done vs to-do.
4. **Execution** — creates other documents (` + "`architecture/`" + `, decisions) and
   develops the project; **updates the task and plan files** in place.
5. **Final reports** — ` + "`worklog/`" + `, ` + "`notes/`" + `, etc.

Plan and task files are updated during execution (not append-only); decisions and
` + "`architecture/`" + ` follow their own rules (` + "`instructions/decision.md`" + `,
` + "`instructions/architecture.md`" + `).
`

const instrTasksTemplate = `# Task Files (one per plan phase)

` + "`context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md`" + ` holds the task list for **one phase of a plan**.
There is no single global TODO: each plan phase gets its own file so the agent
can track exactly what is done and what is pending for that phase. The task file
is the **execution unit**: work happens from one task file at a time, never from
the plan.

## Intent first

Task files **follow the plan**; the plan **follows the analysis**. Do not start
implementation without the analysis → plan → tasks chain for a new objective,
and take each step — plan, tasks, execution — only with **explicit user
approval** unless the user indicated to proceed. If the intent lies inside an
existing analysis/plan, continue that chain (integrate/modify) instead of
restarting; never modify a previous analysis/plan on your own.

## Structure

` + codeFence + `markdown
---
kind: tasks
summary: "<1-2 sentence summary — MANDATORY, index source>"
objective: "<plan phase this task file belongs to>"
status: pending          # pending | in-progress | completed | archived
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - plan/<date>-<plan-slug>.md
sources:
  - plan/<date>-<plan-slug>.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

- [ ] step one
- [~] step two (in progress)
- [x] step three (done)
- [!] step four (blocked)
` + codeFence + `

## Checklist item detail

Each checklist item should be a single, self-contained action. For items that
need more context, use a description indented below the checkbox:

` + codeFence + `markdown
- [ ] Refactor auth middleware to use token pool
      File: ` + "`internal/auth/middleware.go`" + ` — replace per-request
      token allocation with a sync.Pool; must not change the public API.
- [~] Add unit tests for token pool
      Must cover: concurrent access, pool exhaustion fallback.
      Verification: ` + "`go test ./internal/auth/ -race`" + `
- [ ] Update migration script
      Depends on: refactor auth middleware.
` + codeFence + `

Keep descriptions concise: what changes, which files, what to verify. Avoid
full paragraphs — the task file's Verification commands and Definition of Done
sections handle that.

## Frontmatter properties

| Property | Mandatory | Description |
|----------|-----------|-------------|
| kind | yes | Document type, fixed value ` + "`tasks`" + `; index and lint depend on it. |
| summary | yes | 1-2 sentence summary → index source. |
| objective | yes | Plan phase this task file belongs to. |
| status | yes | File state: ` + "`pending`" + ` (to work on) / ` + "`in-progress`" + ` / ` + "`completed`" + ` / ` + "`archived`" + `. The first three are always distinguishable — a task file is never stuck with fewer than three meaningful states. ` + "`active`" + ` is the legacy value accepted by lint. |
| created | yes | ISO 8601 creation date. |
| updated | yes | ISO 8601 last-edit date; refresh on every change. |
| links | yes | Relative path to the plan this task file executes. |
| project | yes | Project id from ` + "`.sdt.yaml`" + ` (e.g. ` + "`sdt_44f24890`" + `). |
| agent | no | Agent/tool that picked up or last edited the file. |
| model | no | Model id of that agent — provenance. |
| session | no | Session id for traceability across edits. |

## Rules

- Status markers: ` + "`[ ]`" + ` todo · ` + "`[~]`" + ` in-progress · ` + "`[x]`" + ` done · ` + "`[!]`" + ` blocked.
- Manage with ` + "`sdt context task <sub> --phase <n> [--plan <slug>]`" + ` (add/list/done/block/wip).
- File status transitions: create → ` + "`pending`" + `; ` + "`wip`" + `/` + "`block`" + ` → ` + "`in-progress`" + `; ` + "`done`" + ` → ` + "`completed`" + ` when no ` + "`[ ]`" + `/` + "`[~]`" + ` item remains, else ` + "`in-progress`" + `; ` + "`archive`" + ` → ` + "`archived`" + `.
- One plan phase per file: **single focus**, small checklist (~5-7 items); a
  >10-item checklist triggers a ` + "`sdt context lint`" + ` SUGGESTION to split.
- Task files are **living**: updated during execution of the phase.
- When a phase completes, archive or remove its task file and update the plan.

## Execution workflow

1. **Pick** — choose a task file whose dependencies are satisfied (see its plan
   and links). One file at a time; don't jump between files.
2. **Take in charge** — before any work, mark the file in progress: set the
   current item to ` + "`[~]`" + ` and refresh the frontmatter (at least
   ` + "`updated`" + `; record ` + "`agent`" + ` / ` + "`model`" + ` / ` + "`session`" + ` when
   available). Log the take-in in the worklog.
3. **Execute** — work items top to bottom; update markers as you go. Any
   deviation goes back into this file or the plan — never silently.
4. **Complete** — ` + "`[x]`" + ` every item, run the **verify-step** below, then
   archive or remove the file and update the linked plan phase.

## Stale task files

At the start of any execution pass, scan ` + "`context/tasks/`" + ` for stale
files: status ` + "`in-progress`" + ` (or legacy ` + "`active`" + `) with ` + "`[~]`" + ` in-progress items that went
unupdated for a long time (compare each file's ` + "`updated`" + ` with the current
date and the recorded take-in). For each stale file:

1. Receive it and adjust status: locate the exact point of work from
   ` + "`updated`" + ` and the ` + "`[~]`" + ` markers.
2. Verify against the linked plan and worklog whether work continued elsewhere
   or stalled.
3. If stalled: reset in-progress items to ` + "`[ ]`" + ` (todo) or close them
   ` + "`[x]`" + ` when actually finished; clear all ` + "`[~]`" + `; refresh
   ` + "`updated`" + `.
4. If the phase is finished but not closed: mark items ` + "`[x]`" + `, archive the
   file, update the plan, note it in the worklog.

Never leave ` + "`[~]`" + ` in-progress markers unattended across sessions.

## Verify-step

Before closing a phase run the verify-step: completeness (all steps handled),
coherence (documents agree), correctness (no broken links / stale state).
Prioritize CRITICAL / WARNING / SUGGESTION and degrade gracefully.

Use a standardized claim vocabulary for checklist items: **passed** (ran and
verified), **expected** (written, not run), **inferred** (static analysis only).
Never mark ` + "`[x]`" + ` a claim that did not run — use a description instead.

At session end: note open tasks and questions in ` + "`context/tasks/`" + ` or
` + "`context/questions/`" + ` for continuity.
`

const instrDecisionTemplate = `# Decision Records

` + "`context/decisions/NNNN-<slug>.md`" + ` records a decision and its rationale.
Numbered with 4 digits (0001, 0002, ...), in chronological order, append-only.
A new decision creates a new record with the next number; existing decision
records are never rewritten in place.

## Structure

` + codeFence + `markdown
---
kind: decision
number: NNNN
title: "<one-line title>"
summary: "<1-2 sentence summary — MANDATORY, index source>"
status: proposed   # proposed | accepted | rejected | deprecated | superseded
created: "<ISO 8601>"
links:
  - architecture/stack.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

# NNNN. <Title>

## Status
## Context
## Decision
## Alternatives considered
### Alternative A: <name>
### Alternative B: <name>
## Consequences
### Positive
### Negative / trade-offs accepted
### Follow-up required
## Notes
` + codeFence + `

## Section descriptions

- **Status** — ` + "`proposed`" + ` / ` + "`accepted`" + ` / ` + "`rejected`" + ` / ` + "`deprecated`" + ` /
  ` + "`superseded by NNNN`" + `. State it plainly as text, not only comments.
- **Context** — the forces at play: technical, business, team constraints. What
  problem is being decided and why now. State facts, not the decision itself yet.
  A reader unfamiliar with the situation should understand why this decision was
  necessary.
- **Decision** — a clear, direct sentence: "We will use X to do Y." Not a
  discussion — that belongs in Context and Alternatives.
- **Alternatives considered** — one subsection per rejected option with pros /
  cons / why it was rejected. Include the leading runner-up even if clearly
  worse, so future readers don't re-litigate it.
- **Consequences** — what becomes easier or harder. Be honest about trade-offs;
  a decision record that only lists benefits isn't trustworthy. Split into
  **Positive**, **Negative / trade-offs accepted** and **Follow-up required**
  (checkboxes, e.g. update ` + "`architecture/`" + ` diagrams, migrate old usages).
- **Notes** — optional: links to benchmarks, discussion threads, prior art,
  related decisions.

## Rules

- Append-only and incremental: a revision of a past decision is a NEW record
  with a higher number (which may mark the old one ` + "`status: superseded`" + `).
- The ` + "`number`" + ` must match the filename prefix (` + "`NNNN-`" + `).
- When a decision changes the architecture, **sync it into the living
  ` + "`architecture/`" + ` document** (merge intelligently: preserve untouched
  content, re-read from disk) — delta→main.

## Tier

` + "`decisions/`" + ` and ` + "`architecture/`" + ` are the **essential** tier: always
versioned, read at session start after ` + "`index.md`" + `.
`

const instrArchitectureTemplate = `# Architecture (living documents)

` + "`context/architecture/`" + ` holds the living architecture documentation:
` + "`stack.md`" + `, ` + "`map.md`" + `, ` + "`flows.md`" + `, ` + "`components.md`" + `, ...

## Structure

One file per system/component. Every file is self-sufficient: assume the reader
has zero prior context.

` + codeFence + `markdown
---
kind: architecture
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what this architecture covers and why>"
status: current            # draft | current | superseded
component: "<slug>"        # e.g. config-loading, task-index
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:                     # related docs (decisions it depends on)
  - decisions/0001-config
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

# Architecture: <System / Component>

## Context
## Goals
## Non-goals
## High-level overview        (ASCII or mermaid diagram)
## Components
### <Component 1>
### <Component 2>
## Data flow
## Technology choices         (table)
## Quality attributes
## Constraints
## Alternatives considered
## Evolution / migration notes
## Open risks
` + codeFence + `

## Section descriptions

- **Context** — what this system/component does and why it exists, in a few
  sentences. Self-sufficient: no assumed prior knowledge.
- **Goals** — quality attributes or capabilities it must deliver (e.g.
  "sub-200ms p95 query latency").
- **Non-goals** — explicitly out of scope; as important as goals, prevents scope
  creep and misaligned expectations.
- **High-level overview** — short narrative followed by a diagram. Use a
  text-based diagram (ASCII or mermaid) so it stays diffable in git and
  readable by agents without image rendering.
- **Components** — one subsection per component: **Responsibility** (what it
  owns), **Depends on** (other components), **Key files/packages**
  (` + "`path/to/pkg`" + `).
- **Data flow** — how data moves through the system for the main use case(s).
  Sequence-like description is often clearer than a diagram.
- **Technology choices** — a table: Choice | Rationale (short) | decision. Link the
  full decision record for anything non-obvious; don't re-argue it here.
- **Quality attributes** — cross-cutting concerns: Scalability, Security,
  Observability, Failure modes. Only include rows actually relevant.
- **Constraints** — things that shaped the design and aren't up for debate in
  scope: infra limits, org policy, external dependencies.
- **Alternatives considered** — high-level rejected alternatives with why; a
  summary/index, details go in the related decision record.
- **Evolution / migration notes** — how this architecture is expected to change,
  or how a past one migrated to this. Useful context for anyone tempted to
  "fix" a deliberate trade-off.
- **Open risks** — architectural risks not yet mitigated (e.g. single point of
  failure at X).

## Rules

- **Naming**: kebab-case, **no date** in the filename.
- **Living**: update in place when the architecture changes; updates can come
  from new decisions (sync — see ` + "`instructions/decision.md`" + `).
- **Mermaid**: embed diagrams in the markdown where useful and not too large; if
  large use separate ` + "`.mmd`" + ` files; an optional ` + "`architecture.mmd`" + `
  holds the whole-architecture graph.
- Keep frontmatter: ` + "`kind: architecture`" + `, mandatory ` + "`summary`" + `, optional
  ` + "`links`" + `.

## Tier

` + "`architecture/`" + ` and ` + "`decisions/`" + ` are the **essential** tier: always
versioned, read at session start after ` + "`index.md`" + `.
`

const instrWorklogTemplate = `# Work Logs

` + "`context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` append one dated entry per
completed change: what changed and why. Append-only history; do not rewrite
existing entries.

## Structure

` + codeFence + `markdown
---
kind: worklog
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what triggered this entry>"
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - plan/<date>-<plan-slug>.md
  - tasks/<date>-<plan-slug>-phase-1.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

## <YYYY-MM-DD HH:MM> — <session title>

- **Actor**: <agent-id / human name>
- **Task ref**: <task file, if applicable>
- **Plan ref**: <plan file, if applicable>
- **Duration**: <approx, optional>

**Summary**

<1-3 sentences: what was actually done this session.>

**Changes made**

- <file/module touched, one line each>

**Decisions made**

- <small local decision>: <why>   (architectural → its own decision record, linked here)

**Deviations from plan/task**

- <deviation, and reason>         (silent deviations are the main source of
                                   plan/reality drift — say so explicitly)

**Blockers encountered**

- <blocker> → <resolved how / still open>

**Next steps**

- <what should happen next, if not already in a task file>
` + codeFence + `

## Section descriptions

- **Actor / Task ref / Plan ref / Duration** — metadata block identifying who
  (or which agent) worked, on what task and plan, for how long.
- **Summary** — 1-3 sentences on what was actually done.
- **Changes made** — one line per file/module touched.
- **Decisions made** — only small, local decisions that don't warrant a full
  decision record; anything architecturally significant becomes a decision record and is linked here.
- **Deviations from plan/task** — say explicitly when what was done differs from
  the task/plan. This is what keeps plan/task files trustworthy.
- **Blockers encountered** — the blocker and how it was resolved (or that it's
  still open, pointing to task status).
- **Next steps** — what should happen next, if not already captured in a task
  file.

## Rules

- Append-only (nothing is edited retroactively); new entries are new dated
  entries.
- Entry per change (final report phase of the 5-stage cycle).
- Tier: **low** (history) — indexed for traceability, does not drive action.
`

const instrNotesTemplate = `# Notes

` + "`context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold free-form annotations that
do not fit plan/web log/task files.

## Structure

Low ceremony: frontmatter header + free-form body.

` + codeFence + `markdown
---
kind: notes
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what triggered this note>"
created: "<ISO 8601>"
project: <project>
---

<free-form content>
` + codeFence + `

## Rules

- Anything ephemeral-but-useful that is not a plan, analysis, worklog entry or
  task step.
- Tier: **medium** (notes with context), indexed.
`

const instrQuestionsTemplate = `# Open Questions

` + "`context/questions/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` collect open questions and
unresolved points that need the user's decision before a piece of work can
proceed. Analysis and plan documents must NOT contain open points: surface them
here instead.

## Structure

` + codeFence + `markdown
---
kind: questions
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<objective / what triggered the open points>"
status: active            # active | resolved
sources:                  # document(s) where the points surfaced
  - analysis/<date>-<slug>.md
created: "<ISO 8601>"
updated: "<ISO 8601>"
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
---

## Open questions

One subsection per question (Q1, Q2, ...). Reserve this file for decisions
that would be expensive to get wrong or reverse; don't put low-stakes,
obvious-answer questions here.

<summary checklist, one line per question>

- [ ] Q1: <question> — decision needed by <who>, blocks <what>

<detail per question>

### Q1: <Short question title>

- **Status**: open            # open | answered
- **Raised**: <YYYY-MM-DD> by <agent-id / human>
- **Confidence without an answer**: low   # the assumption this replaced had
                                           # confidence: low, or the action is
                                           # irreversible/costly
- **Blocks**: <task file / plan phase / free text>
- **Context**: <why this question exists — what triggered it>
- **Impact if unanswered**: <what happens if it stays open — e.g. "task-03
  cannot start", "architecture decision deferred">
- **Options**:
  1. <option 1> — <brief implication>
  2. <option 2> — <brief implication>
- **Suggested default**: <assumed if no answer arrives; only if a safe default
  genuinely exists, else "no safe default, hard blocker">

**Answer**: _(leave empty until answered)_

**Answered by / date**: _(fill in when answered)_
` + codeFence + `

## Section descriptions

- **Summary checklist** — one line per open question for a quick status view,
  kept in sync with the detailed entries below.
- **Per-question detail** — each question carries its own Status; Raised (by
  whom); Confidence without an answer (the cost of guessing); what it Blocks;
  Context (why it exists); Impact if unanswered; Options with implications; and
  a Suggested default that makes the cost of silence explicit (omit when there's
  no safe default). Answers are recorded in place, not in new files — answered
  questions stay as a record of decisions made.

## Rules

- Keep no open points in analysis/plan documents: ask on the fly or register the
  question here and prompt the user to answer.
- The ` + "`sources`" + ` frontmatter array links back to the document(s) where each open
  point surfaced (bidirectional traceability: from the questions doc you find the
  analysis, and from the analysis you find its open points).
- Help the user cover the open points, but keep the user in control of the
  decisions. Once answered, resolve the point in the analysis/plan and mark the
  question resolved (` + "`status: resolved`" + `).
- Tier: **medium**, indexed.
`

const instrScriptsTemplate = `# Scripts (context/scripts/)

` + "`context/scripts/`" + ` is a **dynamic, user/agent-owned workspace** for
executable utility scripts created during work and kept for reuse. It is not a
library to memorize and not generated content: scripts are dropped here when
they are written, listed in the index, and invoked on demand.

## When to add a script

- A workflow needs a repeatable step that is easier as a script (parse,
  transform, orchestrate, generate) than as typed commands.
- The logic is likely to be useful again later — even outside the current task:
  this is the reuse-forward home for them.
- Do NOT add one-off throwaway commands here; use ` + "`context/tmp/`" + ` for
  experiments and only promote a script here when it earns its place.

## Conventions

- **Naming**: descriptive, lowercase, hyphenated (` + "`split-catalog.sh`" + `,
  ` + "`sync-translations.sh`" + `). No timestamps — scripts are living tools,
  not dated records like plans/worklogs.
- **Multiline scripts**: store in ` + "`context/scripts/`" + ` files; single-purpose
  commands can be documented here for reuse.
- **Self-documenting**: the first lines must explain the script and its usage:
  a purpose line, usage example, and any required inputs/environment.
- **Executable**: set the executable bit when appropriate; run with a shell
  explicit in the shebang.

## Index (mandatory registration)

Every script must be registered in ` + "`context/scripts/index.md`" + ` at creation
time. Registering means adding one row to the ` + "`Available scripts`" + ` table:
name, purpose, example invocation. Never add a script without registering it —
an unregistered script is unknown to every future session.

Index workflow:

1. Write the script in ` + "`context/scripts/`" + `.
2. Add/update its row in ` + "`context/scripts/index.md`" + `.
3. Rename or remove the row when the script is renamed or deleted.

## Execution

- Read ` + "`context/scripts/index.md`" + ` to find a script; execute it on demand —
  do not read the whole scripts into context.
- The AGENTS.md On-action table route is informed by this file: seeing the
  ` + "`context/scripts/`" + ` column, read this instruction file first.
`

const instrWikiTemplate = `# Wiki Pages (context/wiki/)

` + "`context/wiki/`" + ` holds knowledge-graph pages distilled from ingested
sources. Read this file when **writing or updating** a wiki page — including
plain editing of an existing page, not only the full ingestion pipeline (that
pipeline is owned by ` + "`context/instructions/ingestion.md`" + `).

## Page contract

Every page keeps the stable machine envelope (frontmatter, provenance,
relations, citations — see ` + "`ingestion.md`" + `):

- ` + "`kind`" + `: wiki · ` + "`id`" + `: relative subpath · ` + "`title`" + `: unique ·
  ` + "`type`" + ` (` + "`concept`" + `/` + "`entity`" + `/` + "`decision`" + `/` + "`pattern`" + `/` + "`module`" + `) ·
  ` + "`status`" + ` (` + "`draft`" + `/` + "`active`" + `/` + "`archived`" + `) · ` + "`summary`" + ` ·
  ` + "`relations`" + ` (closed verb vocabulary) · ` + "`tags`" + ` · ` + "`sources`" + `.
- Body anatomy is **adaptive**: the page body takes the form that best fits the
  material (prose, procedure, comparison, timeline...). ` + "`## Summary`" + ` (3-10
  line TL;DR), ` + "`## Claims`" + ` (numbered, atomic, source-cited
  ` + "`(refs/<file>@<sha>:<lines>)`" + `, anchored ` + "`{#claim-<n>}`" + `) and
  ` + "`## Notes`" + ` (rationale, trade-offs, open points) remain the common
  **default**, never a forced template.
- Machine surface = frontmatter + claim anchors + typed wiki-links +
  markdown-ld; human surface = Summary + prose.

## Reading and writing

- Frontmatter values stay consistent with the shared schema; ` + "`id`" + ` equals the
  relative subpath (` + "`wiki/backend/auth.md`" + ` → ` + "`backend/auth`" + `).
- Preserve the graph when editing: relation direction (this node → target),
  closed vocabulary, and ` + "`sources`" + ` entries remain stable unless a source
  adds authority to change them.
- One concept, one page. A page is exhaustive for its concept; material spanning
  several distinct concepts is split into child/sibling nodes wired via
  ` + "`part_of`" + `/` + "`depends_on`" + `/` + "`refers_to`" + `.
- Never overwrite an existing claim silently. Corrections refine
  ` + "`refines #claim-<n>`" + `; conflicting claims stay visible with source, scope,
  and date until reviewed.

## Correlation and relation discipline

- **Ordinary navigation** (prose or ` + "`[[id|label]]`" + `) is not a graph edge;
  only ` + "`relations`" + ` frontmatter with a closed verb creates one.
- A typed relation needs evidence: a claim in the source, or an existing page
  statement naming the link. Co-occurrence alone never justifies a relation.
- ` + "`supersedes`" + `/` + "`conflicts_with`" + ` are graph signals that trigger human
  review; lint checks only their syntax and targets, not the interpretation.

## Padding and quality

- No empty headings, boilerplate restating frontmatter, or copied source
  structure. The page follows the source only when that order is the best
  explanation.
- Do not repeat a linked page's content — state the connection and point to the
  target.

## Verification

- Run ` + "`sdt context wiki lint`" + ` after every change and fix issues caused by
  the edit; run ` + "`sdt context reindex`" + ` when done.
- Keep the page within the concept budget (~120 lines / ~20 claims); a larger
  page is a split signal, not a command to bloat prose.
`

const scriptsIndexTemplate = `---
kind: scripts
summary: "Index of utility scripts in context/scripts/ (name · purpose · example). Keep the table in sync: add a row for every script, remove it when deleted."
---

# Scripts Index

Available utility scripts under ` + "`context/scripts/`" + `. Read this index to
find a script (name, purpose, example), then execute it on demand. General
guidance on when/how to add scripts: see ` + "`context/instructions/scripts.md`" + `.

## Available scripts

| Script | Purpose | Example |
|--------|---------|---------|
| _none yet_ | — | — |

## How to add a script

1. Write the script in ` + "`context/scripts/`" + ` (descriptive lowercase
   name, no timestamp; self-documenting first lines).
2. Register it here: add a row under Available scripts (name, purpose, example
   invocation).
3. Rename or delete rows when scripts change — the index must never lie.
`

const instrReferenceTemplate = `# SDT — Command Reference

SDT (Smart Developer Tools) is a pure-Go, offline-first CLI for AI agents.
Every command is deterministic and machine-readable.

## Input / Output

- Input: stdin | ` + "`--input \"string\"`" + ` | ` + "`--file path`" + ` | ` + "`--inb64`" + `
- Output: ` + "`--format text|json|yaml`" + ` (default text)
- ` + "`--quiet`" + ` suppresses informational output; ` + "`--no-color`" + ` disables ANSI
- Errors: message to stderr + non-zero exit code

## Discover commands

The full, always-current command reference is generated, not written by hand:

` + codeFence + `
sdt manifest --format json            # full command tree
sdt schema --command "<command>"      # JSON Schema for one command
sdt context docs                      # per-command docs in context/sdtdocs/
sdt docs                              # full markdown docs per command (humans)
sdt <command> --help                  # usage for a single command
` + codeFence + `

## Project configuration

` + "`.sdt.yaml`" + ` holds project identity and is found by walking up from the
current directory:

` + codeFence + `yaml
project: myapp_7f2b39e1
group: platform
` + codeFence + `

Create it with ` + "`sdt agent init --project myapp --group platform`" + ` or
` + "`sdt config init --project myapp`" + `; inspect with ` + "`sdt config show`" + `.

## Context knowledge

Documents under ` + "`context/`" + ` are the project knowledge. Per-type
instructions and templates in ` + "`context/instructions/`" + ` (analysis, plan,
tasks, decision, architecture, worklog, notes, questions, proposals, research, prompts, project, scripts, cli
usage). Index and checks:
` + "`sdt context reindex`" + ` / ` + "`sdt context lint`" + ` / ` + "`sdt context status`" + ` /
` + "`sdt context template --type <tipo>`" + `. Agent instruction contract:
` + "`sdt agent verify`" + `. Utility scripts live in ` + "`context/scripts/`" + `
(usage conventions in ` + "`instructions/scripts.md`" + `, inventory in
` + "`scripts/index.md`" + `; executed on demand, never read into context).
Nothing is written by the CLI: the agent edits the Markdown files.

## CLI usage & examples

The curated command catalog, global flags and practical examples live in
` + "`context/instructions/cli.md`" + `.
`

const instrCLITemplate = `# CLI Usage & Examples

Curated command catalog: the main commands grouped by area, with conventions
and practical examples. Hand-maintained; for the complete, always-current
reference use the generated docs:

` + codeFence + `
sdt manifest --format json           # full command tree
sdt schema --command "<command>"     # JSON Schema for one command
sdt context docs                     # per-command docs in context/sdtdocs/
sdt <command> --help                 # usage for a single command
` + codeFence + `

## Conventions

- Input: stdin | ` + "`--input \"<string>\"`" + ` | ` + "`--file <path>`" + ` | ` + "`--inb64 <base64>`" + `
- Output: ` + "`--format text|json|yaml`" + ` (default text); errors to stderr + non-zero exit
- ` + "`--quiet`" + ` suppresses informational output; ` + "`--no-color`" + ` disables ANSI
- Project identity: ` + "`--project`" + ` / ` + "`--group`" + ` flags or ` + "`.sdt.yaml`" + ` (found walking up)

## Agent tooling

- ` + "`sdt context new|path|list|task|docs`" + ` — plan/analysis/worklog/notes, task list, generated docs
- ` + "`sdt template --tmpl`" + ` — render Go templates from JSON/YAML data
- ` + "`sdt extract --type urls|emails|ips|json-blocks|code-blocks|dates`" + `
- ` + "`sdt env parse|get|set|merge`" + ` — .env handling
- ` + "`sdt diff --a A --b B --diff-format unified|json-patch`" + `

## Encoding & hashing

- ` + "`sdt b64|b32|b64url|hex|url|html [dec]`" + ` — encode/decode
- ` + "`sdt sha256|sha1|sha384|sha512|md5`" + ` — hashes
- ` + "`sdt bcrypt|bcrypt verify`" + `, ` + "`sdt hmac --key`" + `, ` + "`sdt keypair`" + `, ` + "`sdt sign|verify`" + `, ` + "`sdt cert inspect|expiry`" + `

## Data & conversion

- ` + "`sdt conv --in json|yaml|toml|csv|msgpack --out ...`" + `
- ` + "`sdt json pretty|minify|valid`" + `

## IDs, time, strings

- ` + "`sdt uid v4|nano|ks`" + `, ` + "`sdt time unix|iso|http`" + `
- ` + "`sdt string uppercase|lowercase|titlecase|count|escape|unescape|replacespace`" + `
- ` + "`sdt regexp|regexp replace --expression`" + `

## Network

- ` + "`sdt http`" + `, ` + "`sdt ipinfo`" + `, ` + "`sdt nslookup`" + `, ` + "`sdt dns --host --type A|AAAA|MX|TXT|CNAME|NS|PTR`" + `, ` + "`sdt port`" + `

## Other

- ` + "`sdt gzip|gunzip`" + `, ` + "`sdt password`" + `, ` + "`sdt qrcode|qrcode read`" + `, ` + "`sdt totp uri|code|verify`" + `, ` + "`sdt vman`" + `, ` + "`sdt config get|set`" + `, ` + "`sdt version`" + `

## Examples

` + codeFence + `
echo "hello" | sdt b64                                  # encode
echo "password" | sdt sha256                            # hash
echo "payload" | sdt hmac --key "secret"                # hmac
echo '{"a":1}' | sdt conv --in json --out yaml          # convert
echo '{"user":"Alice"}' | sdt template --tmpl "Hi {{.user}}"
cat llm_response.txt | sdt extract --type urls
cat config.yaml | sdt conv --in yaml --out json
echo "password" | sdt sha256 | sdt hex                  # pipeline
sdt diff --a old.json --b new.json --diff-format json-patch
sdt totp code --secret BASE32SECRET
sdt dns --host example.com --type A --format json
` + codeFence + `
`

const instrProposalTemplate = `# Proposal Documents

` + "`context/proposals/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` records a proposal before
an implementation or architectural decision. Read this file when creating or
reviewing a proposal.

## Contract

Every proposal starts with frontmatter:

` + codeFence + `yaml
kind: proposal
title: "One-line proposal title"
summary: "1-2 sentence index summary — MANDATORY"
context: "Problem or opportunity"
status: draft # draft | review | accepted | rejected | superseded
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - analysis/<source-analysis>.md
sources:
  - refs/<evidence>.md
project: <project>
` + codeFence + `

## Template

Use these sections: Problem statement, Goals, Non-goals, Constraints, Current
state, Proposed design, Alternatives considered, Impact and migration,
Validation/evidence, Decision outcome, and Follow-up. Separate observed facts
from the proposed choice and preserve links to analyses, procedures, prompts,
and immutable evidence under ` + "`context/refs/`" + `.

## Proposal → decision workflow

An accepted proposal becomes a decision only through a numbered decision record.
Follow this flow after the proposal reaches ` + "`status: accepted`" + `:

1. **Acceptance review** — confirm the proposal is complete: no open points,
   alternatives considered, validation/evidence present.
2. **Create the decision record** — run
   ` + "`sdt context new --type decision --number <NNNN> --title \"<decision title>\"`" + `
   (or omit ` + "`--number`" + ` to auto-assign the next NNNN). Write the decision,
   recording the accepted alternative and its rationale.
3. **Link both ways** — the decision record lists the proposal in ` + "`sources`" + ` and
   ` + "`links`" + ` (` + "`proposals/<date>-<slug>.md`" + `); the proposal records the decision
   in ` + "`links`" + ` and sets ` + "`status: accepted`" + ` (or ` + "`rejected`" + `/` + "`superseded`" + `
   for the other outcomes).
4. **Architectural impact** — when the decision changes the current system
   shape, update the relevant living ` + "`context/architecture/`" + ` document in the
   same execution phase and link it to the decision record. Non-architectural
   decisions may be recorded by the decision record alone.
5. **Reconcile** — run ` + "`sdt context reindex`" + ` and ` + "`sdt context lint`" + `; verify
   the proposal → decision → architecture chain is clean before closing the phase.

Proposals propose; decision records decide; architecture documents describe the
current state. Do not treat research in ` + "`refs/`" + ` or a proposal status alone as
an accepted decision.
`

const instrResearchTemplate = `# Research Documents

` + "`context/research/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` records the results of one
research run (deepsearch, web/document sweep, spike) and its provenance. Read
this file when running or writing up research that feeds an analysis, proposal
or decision.

## Contract

Every research note starts with frontmatter:

` + codeFence + `yaml
kind: research
title: "One-line research title"
summary: "1-2 sentence index summary — MANDATORY"
subject: "The question this run answers"
context: "Why this research was requested"
status: draft # draft | active | archived
created: "<ISO 8601>"
updated: "<ISO 8601>"
sources:            # provenance: the driving prompt + evidence refs
  - prompts/<driving-prompt>.md
  - refs/<evidence>.md
project: <project>
agent: <agent/tool>          # optional
model: <model id>            # optional
session: <session id>        # optional
` + codeFence + `

Body sections: Subject, Method, Findings, Evidence, Limits and open points,
Feeds.

## Purpose and boundary

- **Research = raw results, not a decision.** A research note gathers and
  organizes evidence (with links) so an analysis or proposal can reason over
  it — it never states an accepted choice.
- **Research feeds analyses and proposals.** Link forward with
  ` + "`links: analysis/…, proposals/…`" + ` once the consuming document exists; a
  proposal or decision cites the research in its ` + "`sources`" + `.
- **Boundary vs analysis.** An analysis interprets evidence to reach a
  conclusion/recommendation; research merely collects and attributes it. When
  interpretation starts, write/extend an analysis (or proposal) instead.
- **Provenance is mandatory.** Every run names its driving prompt
  (` + "`prompts/<…>.md`" + `) and the evidence it relied on under ` + "`sources`" + `.
  Non-markdown evidence is converted first (anydoc, then docling — see
  ` + "`instructions/ingestion.md`" + `) and the original kept in ` + "`context/refs/`" + `.

## Deepsearch readiness

- One run → one dated file, so a future ` + "`sdt deepsearch`" + ` integration can
  append machine-produced runs and link them like any other source.
- Keep findings atomic and citable (claim + source + scope + date) so a
  downstream analysis or the wiki can reference them without re-reading the run.
- Prefer ` + "`refs/`" + ` for immutable raw captures; the research note summarizes
  and links, it does not duplicate the raw payload.
`

const instrIngestionTemplate = `# Ingestion → Wiki → Refs (knowledge pipeline)

The three-stage knowledge pipeline on top of ` + "`context/`" + `: source material lands
in ` + "`context/ingestion/`" + `, an **agent command** distils it into knowledge-graph
pages under ` + "`context/wiki/`" + `, and consumed sources are archived to
` + "`context/refs/`" + `. No CLI ingests files — placement is manual (user),
distillation is agent work. This file is the **agent command**: read it when a
new ingestion request arrives.

## Files & lifecycle

| Dir | Role | Owner | Kind | Status |
|---|---|---|---|---|
| ` + "`context/ingestion/`" + ` | raw source awaiting distillation | user (manual) | ` + "`reference`" + ` | ` + "`pending`" + ` |
| ` + "`context/wiki/`" + ` | knowledge-graph pages | agent | ` + "`wiki`" + ` | ` + "`active`" + `/` + "`draft`" + `/` + "`archived`" + ` |
| ` + "`context/refs/`" + ` | archived, immutable sources | agent (after approval) | ` + "`reference`" + ` | ` + "`archived`" + ` |

> Marker enforcement: ` + "`sdt context wiki lint`" + ` enforces the ` + "`ingestion/`" + ` markers
> (` + "`kind: reference`" + `, ` + "`status: pending`" + `). ` + "`refs/`" + ` is **excluded from lint** — its
> markers are a convention only, since it holds large immutable external
> captures that need not follow the frontmatter contract.

` + codeFence + `
user drops files → ingestion/ (pending)
      │
      ▼ agent command: distil → write/update wiki/ pages
      │                                      │
      ▼ (user approves write + archive plan) ▼
agent moves sources → refs/ (archived)   sync node status
` + codeFence + `

- ` + "`ingestion/`" + ` and ` + "`refs/`" + ` are **immutable**: never edit, merge or refactor a
  file there. The only legal mutation is the archive move (pending→archived)
  performed by this command after approval. Anything else is a git-visible
  violation.
- Read-only: facts pulled from ` + "`refs/`" + ` must cite ` + "`file@SHA:lines`" + ` or the target
  node id.

## Invocation (scope)

The user triggers ingestion with an **agent command**, opencode slash-command
style, specifying one of:

- ` + "`all`" + ` — every pending file in ` + "`context/ingestion/`" + ` (default when no scope).
- ` + "`<file>`" + ` — a single pending source (path, name or id).
- ` + "`<glob>`" + ` — a specific group of files.

Only files inside the requested scope are considered; already-archived files
are always skipped.

## Operating sequence

1. Resolve the invocation scope (above).
2. List pending (` + "`status: pending`" + `) files in scope.
3. Read each source: convert non-markdown to markdown (see Reading &
   conversion), then split into atomic facts (distillation rules below).
4. Match existing ` + "`wiki/`" + ` nodes by ` + "`title`" + `/` + "`id`" + `/` + "`tags`" + `: **reuse** a
   same-slug + same-concept node (append/update in place, never duplicate),
   or create a new one.
5. Write or update distilled ` + "`wiki/`" + ` pages per the schema, the six invariants
   and the concept budget.
6. Wire ` + "`relations`" + ` + ` + "`tags`" + `; every claim carries its source citation.
7. Present the **write + archive plan** to the user: pages touched (created/
   updated), relations added/removed, and files to archive. **Wait for
   approval**.
8. On approval: archive processed sources to ` + "`context/refs/`" + ` with
   ` + "`status: archived`" + `, append a ` + "`worklog/`" + ` entry, run ` + "`sdt context reindex`" + `.

**Idempotence**: skip ` + "`status: archived`" + ` files; a re-run matches new candidates
against existing nodes instead of re-reading archived sources as new input.
**Existence guard**: before any write to ` + "`wiki/`" + `, check the target path. A
same-slug file with the same concept is **updated in place** (append claims /
` + "`refines #claim-n`" + `), never overwritten silently; creating a new node requires
the slug to be free.

## Reading & conversion (heterogeneous sources)

Sources are frequently not plain markdown — convert first, then distil.

- **anydoc** is the preferred converter (PDF/DOCX/HTML and other formats →
  clean markdown). Use **docling** as the alternative when anydoc yields no
  usable result.
- **Fallbacks** by source type: ` + "`markitdown`" + ` (Office/web), ` + "`marker`" + `
  (PDF-heavy), OCR for scans (e.g. tesseract) when the above yield no usable
  text. Optional wrapper in ` + "`context/scripts/`" + ` (registered in
  ` + "`scripts/index.md`" + `) keeps the invocation reproducible.
- Conversion output is transient: write intermediate markdown to
  ` + "`context/tmp/`" + ` while distilling — never into ` + "`context/ingestion/`" + `
  (immutable) or directly into the wiki. Raw binaries stay untouched in
  ` + "`context/ingestion/`" + ` until the archive step.
- **Images** (diagrams, screenshots, scans): interpret via a vision-capable
  model; distil the facts into the wiki page; optionally archive the original
  in ` + "`context/refs/`" + ` on approval. Never embed binaries in wiki pages — point
  to ` + "`refs/<file>@<sha>`" + ` instead.

## Wiki page schema

**Frontmatter** (the machine reading surface):

| Field | Required | Notes |
|---|---|---|
| ` + "`kind`" + ` | yes | ` + "`wiki`" + ` |
| ` + "`id`" + ` | yes | relative subpath of the file (` + "`wiki/backend/auth.md`" + ` → ` + "`backend/auth`" + `; flat ` + "`wiki/<slug>.md`" + ` → ` + "`<slug>`" + `) |
| ` + "`title`" + ` | yes | unique across all ` + "`wiki/`" + ` pages (ambiguity = lint error) |
| ` + "`type`" + ` | yes | ` + "`concept`" + ` · ` + "`entity`" + ` · ` + "`decision`" + ` · ` + "`pattern`" + ` · ` + "`module`" + ` |
| ` + "`status`" + ` | yes | ` + "`draft`" + ` · ` + "`active`" + ` · ` + "`archived`" + ` |
| ` + "`summary`" + ` | yes | 1-2 lines for the index |
| ` + "`relations`" + ` | yes | ordered map: closed verbs → ` + "`[[id\\|label]]`" + ` |
| ` + "`tags`" + ` | yes | hierarchical (` + "`backend/performance`" + `) |
| ` + "`sources`" + ` | yes | ` + "`refs/`" + ` files this page cites |
| ` + "`verified`" + ` | no | ` + "`false`" + ` for unverified/imported claims |
| ` + "`supersedes`" + ` | no | node id replaced by this page |

**Subdirectories** (optional, one level by default): ` + "`wiki/<context>/<slug>.md`" + `
groups pages by context label — the subdir name is a kebab-case context
label (` + "`backend`" + `, ` + "`infra`" + `, ` + "`product`" + `). ` + "`sdt context wiki lint`" + ` scans
recursively; a page's ` + "`id`" + ` must equal its relative subpath (` + "`backend/auth`" + `),
global ` + "`title`" + ` uniqueness and ` + "`[[id|label]]`" + `/` + "`[[verb::title]]`" + ` resolution
still apply across subdirs. Flat ` + "`wiki/<slug>.md`" + ` pages (` + "`id: <slug>`" + `) are a
valid special case and stay as-is.

**Body** (the human reading surface):

Page anatomy — every ` + "`wiki/`" + ` node keeps the **fixed body structure**:

- ` + "`## Summary`" + ` — a 3-10 line TL;DR restating the knowledge in the KB's own
  words; the primary human reading surface (frontmatter ` + "`summary`" + ` stays 1-2
  lines for the index).
- ` + "`## Claims`" + ` — numbered, **atomic** assertions; each is one fact, each cites
  its source inline ` + "`(refs/<file>@<sha>:<lines>)`" + `, each carries an anchor
  ` + "`{#claim-<n>}`" + ` so other pages can supersede/refine a single claim without
  editing it.
- ` + "`## Notes`" + ` — rationale, trade-offs, open points; opinion kept separate from
  facts. Notes content varies by ` + "`type`" + `.

**Dual readability:** the human path is ` + "`Summary`" + ` + claims-as-bullets + terse
markdown structure; the agent/machine path is the frontmatter (` + "`relations`" + `/
` + "`tags`" + `), claim anchors, typed wiki-links, and markdown-ld — the surface that
lint parses and ` + "`export`" + ` uses. New pages must ship all three body sections.
Pages written before this anatomy was frozen may stay as-is until touched.

**Typed wiki-links** (Obsidian syntax): body ` + "`[[verb::title]]`" + `, relations
` + "`[[id|label]]`" + `. ` + "`verb`" + ` is always one of the closed vocabulary below; titles
must resolve uniquely.

**markdown-ld** (optional): a JSON-LD block for machine-readable typing, used
only where it adds real value (schema.org export, external/real entities).
When present ` + "`wiki lint`" + ` validates JSON well-formedness and
` + "`name`" + ` ↔ ` + "`title`" + `.

**Relation vocabulary — closed set** (extending it is a schema change; a verb
outside the set fails lint):

| verb | semantics (this node → target) |
|---|---|
| ` + "`supersedes`" + ` | replaces target node/claim; reverse derived |
| ` + "`depends_on`" + ` | requires target as prerequisite |
| ` + "`refers_to`" + ` | points to a referenced concept/entity (default body verb) |
| ` + "`refines`" + ` | narrows/corrects target (claim-level via ` + "`#claim-n`" + `) |
| ` + "`implements`" + ` | realizes a spec/decision/record |
| ` + "`conflicts_with`" + ` | stated contradiction (triggers verification) |
| ` + "`part_of`" + ` | component of target |
| ` + "`contains`" + ` | target is part of this node (mirror of ` + "`part_of`" + `) |

Only ` + "`part_of`" + `/` + "`contains`" + ` is bi-directional; other edges keep one canonical
direction (reverse forms derived, never written).

**Concept budget** (invariant 6, kept schema-versioned): a page targets **one
concept**. ` + "`wiki lint`" + ` raises a WARNING on pages exceeding ~120 lines or ~20
claims as a **split signal**: the page likely spans several distinct concepts
or incomparable material — split it into child/sibling nodes wired via
` + "`part_of`" + `/` + "`depends_on`" + `/` + "`refers_to`" + `.

## Distillation rules

**Page anatomy** is the fixed body structure above (frontmatter + ` + "`## Summary`" + `
+ ` + "`## Claims`" + ` + ` + "`## Notes`" + `); the page must be **knowledge, not a copy**.

**The six "not-a-copy" invariants:**

1. **Re-express, never quote.** Own wording; quote only when a downstream
   consumer depends on the exact value, naming the dependent.
2. **Cite, don't paste.** Every claim cites its source; a page must be
   re-derivable from ` + "`refs/`" + ` + git history. Un-expressible nuggets stay in
   ` + "`refs/`" + ` and are cited, not pasted.
3. **One fact, one claim.** Atomicity enables dedup, per-claim refine/
   supersede, and lint checks; the same fact never lives in two nodes.
4. **Shape, not value.** Describe structures/concepts/decisions; live values
   stay in frontmatter or repo files, referenced by pointer.
5. **Pointer, not copy, across nodes.** Cross-references go through
   ` + "`relations`" + ` + typed wiki-links; never restate another node's content.
6. **One concept, one page.** Exhaustive for its concept, never spanning many
   distinct concepts — split instead (see concept budget).

**Transformation algorithm** (per source):

1. Read source in full; split into atomic facts.
2. Group facts into candidate nodes by concept/entity/decision; split any
   candidate spanning several distinct concepts (pre-wire child relations).
3. Match existing ` + "`wiki/`" + ` nodes by ` + "`title`" + `/` + "`id`" + `/` + "`tags`" + `; reuse, split or create.
4. Existing nodes: **append** claims; corrections add ` + "`refines #claim-<n>`" + ` —
   never rewrite history in place.
5. Set ` + "`relations`" + ` + ` + "`tags`" + `; verify every claim has a citation and ` + "`sources`" + `
   lists the refs file.
6. When all scoped files are handled: present plan → user approves → archive.

## Verification

- ` + "`sdt context wiki lint`" + ` — schema/graph checks on ` + "`wiki/`" + ` (scanned
  recursively; ` + "`id`" + ` = relative subpath) plus ` + "`ingestion/`" + ` markers
  (` + "`refs/`" + ` excluded). Separate from the general ` + "`sdt context lint`" + `.
- After every run: ` + "`sdt context reindex`" + `.
`

const instrPromptsTemplate = `# Tracked Prompts

` + "`context/prompts/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` records a reusable or
executed prompt and its provenance. Read this file when creating, revising, or
running a prompt that contributes to an analysis, procedure, proposal, or decision.

## Contract

Every prompt starts with frontmatter:

` + codeFence + `yaml
kind: prompt
title: "Prompt title"
summary: "1-2 sentence index summary — MANDATORY"
status: draft # draft | active | archived
created: "<ISO 8601>"
updated: "<ISO 8601>"
derived_from:
  - analysis/<analysis>.md
procedure: instructions/<procedure>.md
sources:
  - refs/<deepsearch-result>.md
results:
  - analysis/<follow-up>.md
project: <project>
` + codeFence + `

Keep the complete prompt text in the body. Link the analysis, proposal, instruction,
or procedure that produced it through ` + "`derived_from`" + ` and record deep-
search outputs by relative pointers under ` + "`context/refs/`" + `. Do not copy
large reports into the prompt record or modify files in ` + "`refs/`" + `.

## Runs

Use one ` + "`## Runs`" + ` section for repeated executions. Each row records at
least date/time, model or tool, scope, status, and result references. Link
resulting analyses, proposals, decisions, or other documents in ` + "`results`" + ` or the
run row. Split runs into separate files only through a later schema change.
`

// instrCommandsIndexTemplate builds context/commands/index.md: the lookup
// surface mapping each trigger to its command file and durable instruction.
func instrCommandsIndexTemplate(project string, now time.Time) string {
	ts := now.UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString(`---
kind: commands
id: commands/index
title: "Agent-invokable commands index"
status: active
summary: "Lookup surface for agent-invokable commands: maps each trigger (>ingestion) to its command file under context/commands/ and to its durable instruction under context/instructions/. Durable contracts stay in context/instructions/; command files invoke/reference them, never move or duplicate them."
links:
  - commands/ingestion.md
sources:
  - instructions/ingestion.md
project: ` + project + `
created: "` + ts + `"
updated: "` + ts + `"
---

# Agent-invokable commands index

` + "`context/commands/`" + ` holds one thin file per **agent-visible task**, named by
its trigger. Each command file **invokes or references** its durable contract (a
file under ` + "`context/instructions/`" + `); it never moves or duplicates the contract
text.

Invocation method: an **agent command** — opencode slash-command style,
` + "`>trigger`" + `, e.g. ` + "`>ingestion`" + `. The trigger resolves deterministically to
` + "`context/commands/<trigger>.md`" + `, then to the contract under
` + "`context/instructions/<trigger>.md`" + `.

## Registry

| Trigger | Command file | Durable contract |
|---|---|---|
`)
	for _, id := range agentCommandIDs {
		fmt.Fprintf(&b, "| `>%s` | `context/commands/%s.md` | `context/instructions/%s.md` |\n", id, id, id)
	}
	b.WriteString(`
Lookup order for a trigger ` + "`>T`" + `: ` + "`context/commands/T.md`" + ` → if absent, fall
back to ` + "`context/instructions/T.md`" + `. If neither exists, ask the user (never
invent a contract).

## Invocation contract

- **Approve-before-write:** every execution requires user approval before the
  agent writes anything. No approve gate → no write.
- **Approve-before-archive:** moving sources ` + "`ingestion/ → refs/`" + ` happens only
  with user approval.
- **Sealed sources:** ` + "`ingestion/`" + ` and ` + "`refs/`" + ` are immutable; the only legal
  mutation is the archive move after approval.
- **Scope:** ` + "`all`" + ` (default) · single file · glob — see each command file.
`)
	return b.String()
}

// instrCommandStubTemplate builds the thin command file for one trigger: it
// resolves the trigger and delegates to the durable contract.
func instrCommandStubTemplate(id, project string, now time.Time) string {
	ts := now.UTC().Format(time.RFC3339)
	return `---
kind: commands
id: commands/` + id + `
title: ">` + id + ` — agent-invokable task trigger"
summary: "Thin agent command: invoked by the >` + id + ` trigger. Proceeds per context/instructions/` + id + `.md (the durable contract). Approve-before-write gate."
status: active
links:
  - commands/index.md
  - instructions/` + id + `.md
sources:
  - instructions/` + id + `.md
project: ` + project + `
created: "` + ts + `"
updated: "` + ts + `"
---

# ` + "`>" + id + "`" + ` — read this when the command fires

**Do not re-implement the task here.** The durable contract lives in
` + "`context/instructions/" + id + ".md`" + `; this file is the thin trigger.

## Invocation

- Canonical trigger: ` + "`>" + id + "`" + ` (see ` + "`context/commands/index.md`" + `).
- Scope: ` + "`all`" + ` (default) · single file · glob — see the contract for the
  task's scope semantics.
- No settle: **resolve ` + "`context/commands/" + id + ".md`" + ` → read
  ` + "`context/instructions/" + id + ".md`" + ` → proceed per that contract.**

## Gates (non-negotiable)

- **Approve-before-write** — no write before user approval (see the contract).
- Approve-before-archive where the contract moves sources
  (` + "`ingestion/ → refs/`" + `); ` + "`ingestion/`" + `/` + "`refs/`" + ` stay immutable.

## Verify

- Per the contract; ` + "`sdt context wiki lint`" + ` + ` + "`sdt context reindex`" + ` clean after writes.
`
}
