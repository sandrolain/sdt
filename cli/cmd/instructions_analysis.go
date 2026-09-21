package cmd

const instrAnalysisTemplate = `# Analysis Documents

` + "`context/analysis/<YYYYMMDD-HHMMSS>-<slug>.md`" + ` hold investigation and
implementation plans. They are important while open, less once implemented.

## Purpose

- Record a reasoning/decision process: what was examined, what was decided, why.
- Every analysis has a clear objective that triggered it.

## When to start here (intent first)

A new objective outside an existing analysis/plan starts here: create the
analysis and stop, then proceed plan → tasks → execution only with explicit
approval — see AGENTS.md (HARD RULE 1).

## Structure

` + codeFence + `markdown
---
kind: analysis
title: "<one-line title>"
summary: "<1-2 sentence summary — MANDATORY, index source>"
context: "<objectives / what triggered the analysis>"
objective: <slug>         # optional: kebab-case group key; same slug in every
                          # analysis targeting the same objective
topics: [<topic>, ...]    # optional: controlled subjects from context/topics.yaml
entities: [<entity>, ...] # optional: named things the analysis mentions
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

- **No H1 title** — body starts at H2; see AGENTS.md (document conventions).
- ` + "`summary`" + ` is mandatory and used by ` + "`sdt context reindex`" + `.
- Timestamps: ` + "`created`" + `/` + "`updated`" + ` are UTC ISO 8601 — generate them with
  ` + "`sdt time iso`" + `.
- Dated files: integrate/modify the current analysis while it is the active one;
  a materially new line of investigation gets a new dated file (and sets
  ` + "`sources`" + ` back to the analysis it extends).
- ` + "`objective`" + ` (optional): kebab-case slug identifying the initiative; use the same
  slug in every analysis that targets the same objective so
  ` + "`sdt context reindex`" + ` and the viewer can group them.
- **Check the objective's dead ends first**: before opening an analysis for an
  objective, review its ` + "`note_type: dead-end`" + ` notes (` + "`sdt context status`" + `
  counts them; ` + "`sdt context reindex`" + ` groups them under the objective) and do
  not re-run a rejected approach without new evidence.
- ` + "`topics`" + ` / ` + "`entities`" + ` (optional): subjects and named things,
  **distinct from ` + "`objective`" + `**. Topics come from the controlled list in
  ` + "`context/topics.yaml`" + `; ` + "`sdt context lint`" + ` reports unknown topics
  (SUGGESTION, with the canonical form for aliases) and they feed search filters
  and prior-art detection.
- **Find prior art before writing.** ` + "`sdt context new --type analysis --prior-art`" + `
  searches the corpus on the objective/title/topics and proposes candidates in a
  ` + "`## Prior art`" + ` section (add ` + "`--prior-art-links`" + ` to pre-fill ` + "`links`" + `).
  Review every candidate, keep or discard it, and never leave the auto-generated
  note in the final document.
- **Declare how it relates to prior work.** Set ` + "`links`" + ` (generic
  correlation), ` + "`supersedes`" + ` (this document replaces an older one) or
  ` + "`contradicts`" + ` (it rebuts it), or state ` + "`links: none`" + ` with a reason when
  there is genuinely no prior work. ` + "`sdt context lint`" + ` raises a SUGGESTION
  when an analysis declares nothing, and another when two analyses of the same
  ` + "`objective`" + ` have near-identical title/summary (possible duplicate — link
  or differentiate them).
- Leave **no open points**: if a decision is missing, ask on the fly or register
  it as an open question in ` + "`context/questions/`" + `; keep the user in
  control of the decisions.
- Track work via the 5-stage development lifecycle — see AGENTS.md.
- Verify-step before finishing — see AGENTS.md (5-stage development lifecycle).

## Initial scan (bootstrap)

On an established (brownfield) or new (greenfield) project, do an initial scan
before changing anything: read the README, docs, code structure and ` + "`git log`" + `.
Offer what to capture and ask confirmation before writing knowledge files
(non auto-capturing). Populate ` + "`architecture/`" + ` and new decisions in
` + "`decisions/`" + `; ` + "`worklog/`" + `/` + "`archive/`" + ` stay history.
`
