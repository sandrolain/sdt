package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

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

` + "`sdt.context/analysis/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold investigation and
implementation plans. They are important while open, less once implemented.

## Purpose

- Record a reasoning/decision process: what was examined, what was decided, why.
- Every analysis has a clear objective that triggered it.

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
---

# <Title>

## 1. Goal / framing
## 2. Baseline / current state
## 3. Options / deltas (with a decision where one was taken)
## 4. What is NOT useful (rejected, with reason)
## 5. Integration / decisions resolved
## 6. Commitments (prioritized work packages)
## 7. Context notes (nuances not to confuse)
## 8. Outcome
` + codeFence + `

## Rules

- ` + "`summary`" + ` is mandatory and used by ` + "`sdt context reindex`" + `.
- Dated files: integrate/modify the current analysis while it is the active one;
  a materially new line of investigation gets a new dated file (and sets
  ` + "`sources`" + ` back to the analysis it extends).
- Leave **no open points**: if a decision is missing, ask on the fly or register
  it as an open question in ` + "`sdt.context/questions/`" + `; keep the user in
  control of the decisions.
- Track work via the 5-phase cycle: Analysis → Plan → Tasks per phase → Execution
  (updates plan+task, creates architecture/ADRs) → Final reports.
- Verify-step before finishing: completeness, coherence, correctness; prioritize
  CRITICAL / WARNING / SUGGESTION and degrade gracefully.

## Initial scan (bootstrap)

On an established (brownfield) or new (greenfield) project, do an initial scan
before changing anything: read the README, docs, code structure and ` + "`git log`" + `.
Offer what to capture and ask confirmation before writing knowledge files
(non auto-capturing). Populate ` + "`architecture/`" + ` and new ADRs in
` + "`decisions/`" + `; ` + "`worklog/`" + `/` + "`archive/`" + ` stay history.
`

const instrPlanTemplate = `# Plan Documents

` + "`sdt.context/plan/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold the plan for a piece of
work, derived from the related analysis. Plans are **living documents**: they
are updated during execution, not frozen.

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
project: <project>
---

# <Title>

## Objective
## Phases (each maps to a tasks checklist, see below)
## Design / decisions
## Verification
` + codeFence + `

## 5-phase development lifecycle

Every non-trivial piece of work follows this cycle; a plan is phase 2:

1. **Analysis** — perform and integrate/modify the analysis.
2. **Plan** — created from the analysis; integrated/modified as needed.
3. **Tasks** — from the plan create **one checklist file per phase** (see
   ` + "`instructions/tasks.md`" + `) so the agent can track done vs to-do.
4. **Execution** — creates other documents (` + "`architecture/`" + `, ADRs) and
   develops the project; **updates the tasks and plan files** in place.
5. **Final reports** — ` + "`worklog/`" + `, ` + "`notes/`" + `, etc.

Plan and task files are updated during execution (not append-only); ADRs and
` + "`architecture/`" + ` follow their own rules (` + "`instructions/adr.md`" + `,
` + "`instructions/architecture.md`" + `).
`

const instrTasksTemplate = `# Task Checklists (per plan phase)

` + "`sdt.context/tasks/<phase>.md`" + ` holds the checklist for **one phase of a plan**.
There is no single global TODO: each plan phase gets its own file so the agent
can track exactly what is done and what is pending for that phase.

## Structure

` + codeFence + `markdown
---
kind: tasks
summary: "<1-2 sentence summary — MANDATORY, index source>"
objective: "<plan phase this checklist belongs to>"
status: active
created: "<ISO 8601>"
updated: "<ISO 8601>"
links:
  - plan/<date>-<plan-slug>.md
project: <project>
---

- [ ] step one
- [~] step two (in progress)
- [x] step three (done)
- [!] step four (blocked)
` + codeFence + `

## Rules

- Status markers: ` + "`[ ]`" + ` todo · ` + "`[~]`" + ` in-progress · ` + "`[x]`" + ` done · ` + "`[!]`" + ` blocked.
- Manage with ` + "`sdt context task <sub> --phase <phase>`" + ` (add/list/done/block/wip).
- Tasks are **living**: updated during execution of the phase.
- When a phase completes, archive or remove its checklist file.

## Verify-step

Before closing a phase run the verify-step: completeness (all steps handled),
coherence (documents agree), correctness (no broken links / stale state).
Prioritize CRITICAL / WARNING / SUGGESTION and degrade gracefully.
`

const instrAdrTemplate = `# ADR (Architecture Decision Records)

` + "`sdt.context/decisions/NNNN-<slug>.md`" + ` record a decision and its rationale.
Numbered with 4 digits (0001, 0002, ...), in chronological order, append-only.
A new decision creates a new ADR with the next number; existing ADRs are never
rewritten in place.

## Structure

` + codeFence + `markdown
---
kind: adr            # decision
number: NNNN
title: "<one-line title>"
summary: "<1-2 sentence summary — MANDATORY, index source>"
status: active       # accepted | superseded
created: "<ISO 8601>"
links:
  - architecture/stack.md
project: <project>
---

# NNNN. <Title>

## Context
## Decision
## Alternatives considered
## Consequences / blast radius
` + codeFence + `

## Rules

- Append-only and incremental: a revision of a past decision is a NEW ADR with a
  higher number (which may mark the old one ` + "`status: superseded`" + `).
- The ` + "`number`" + ` must match the filename prefix (` + "`NNNN-`" + `).
- When an ADR changes the architecture, **sync it into the living
  ` + "`architecture/`" + ` document** (merge intelligently: preserve untouched
  content, re-read from disk) — delta→main.

## Tier

` + "`decisions/`" + ` and ` + "`architecture/`" + ` are the **essential** tier: always
versioned, read at session start after ` + "`index.md`" + `.
`

const instrArchitectureTemplate = `# Architecture (living documents)

` + "`sdt.context/architecture/`" + ` holds the living architecture documentation:
` + "`stack.md`" + `, ` + "`map.md`" + `, ` + "`flows.md`" + `, ` + "`components.md`" + `, ...

## Rules

- **Naming**: kebab-case, **no date** in the filename.
- **Living**: update in place when the architecture changes; updates can come
  from new ADRs (sync — see ` + "`instructions/adr.md`" + `).
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

` + "`sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` append one dated entry per
completed change: what changed and why. Append-only history; do not rewrite
existing entries.

## Structure

` + codeFence + `markdown
---
kind: worklog
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<what triggered this entry>"
created: "<ISO 8601>"
project: <project>
---

## <date> — <one-line subject>

<what changed and why. Concise technical language.>
` + codeFence + `

## Rules

- Append-only (nothing is edited retroactively); new entries are new dated
  entries.
- Entry per change (final report phase of the 5-phase cycle).
- Tier: **low** (history) — indexed for traceability, does not drive action.
`

const instrNotesTemplate = `# Notes

` + "`sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold free-form annotations that
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

` + "`sdt.context/questions/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` collect open questions and
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
---

## Open questions

- [ ] Q1: <question> — options: A / B / C — decision needed by <who>
- [ ] Q2: <question> — ...
` + codeFence + `

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
sdt context docs                      # per-command docs in sdt.context/docs/
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

Documents under ` + "`sdt.context/`" + ` are the project knowledge. Per-type
instructions and templates in ` + "`sdt.context/instructions/`" + ` (analysis, plan,
tasks, adr, architecture, worklog, notes, questions, project, cli usage). Index and checks:
` + "`sdt context reindex`" + ` / ` + "`sdt context lint`" + ` / ` + "`sdt context status`" + ` /
` + "`sdt context template --type <tipo>`" + `. Nothing is written by the CLI: the
agent edits the Markdown files.

## CLI usage & examples

The curated command catalog, global flags and practical examples live in
` + "`sdt.context/instructions/cli.md`" + `.
`

const instrCLITemplate = `# CLI Usage & Examples

Curated command catalog: the main commands grouped by area, with conventions
and practical examples. Hand-maintained; for the complete, always-current
reference use the generated docs:

` + codeFence + `
sdt manifest --format json           # full command tree
sdt schema --command "<command>"     # JSON Schema for one command
sdt context docs                     # per-command docs in sdt.context/docs/
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
