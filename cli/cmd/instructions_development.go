package cmd

const instrDevelopmentTemplate = `# Development (how to write code)

` + "`context/instructions/development.md`" + ` is the coding contract: it tells the
agent how to agree style and architecture with the user, escalate ambiguity,
prefer existing libraries, and use library documentation. Read it before
planning or writing any non-trivial code.

## Style and architecture are agreed, not invented

- For every non-trivial piece of work, propose — in the plan, before any code —
  the architecture (components, boundaries, patterns) and style (naming, layout,
  idioms, formatting). Get **explicit user approval** before writing code.
- If a new aspect appears mid-work (a new module, a new subsystem), re-run the
  agreement for that aspect; never extend the existing style silently.
- Record the agreed conventions back into the AGENTS.md project block
  (` + "`Conventions`" + ` / ` + "`Stack`" + `) once stable, and keep them in sync.

## Every ambiguity is decided by the user

- Any non-trivial choice — style, architecture, dependency, naming, data model,
  API shape, error handling, logging, build tooling — defaults to the user.
- Ask on the fly; if it needs tracking, register an open question in
  ` + "`context/questions/`" + `. Never pick a "reasonable default" for anything that
  shapes the project.

## Library-first development

- Before writing non-trivial code, evaluate existing libraries:
  1. state the capability needed in one line;
  2. search the web and the local docs under ` + "`context/refs/`" + ` for candidates;
  3. shortlist 2-3 options with maintenance status, license, latest version,
     footprint and API fit;
  4. present the shortlist to the user and ask which to use; "write it ourselves"
     is a valid outcome only with the user's explicit choice and a stated reason.
- Never reinvent what a maintained library already provides. The rule targets
  non-trivial functionality, not one-line helpers.

## Library documentation — local, updated copy

- For every library the project adopts, keep an updated local copy of its docs
  under ` + "`context/refs/<lib>/`" + ` (fetch with
  ` + "`sdt crawldown <docs-url> --output context/refs/<lib>`" + ` or the web-fetch
  tool). Store a version marker (fetched version + date).
- Before using a library, check its latest version (changelog/releases) against
  what you know; APIs change — do not code against a stale API.
- For an unfamiliar library, learn usage from its docs, not by reading its source.
- Refresh the local docs when the version changes, or when unsure about the API.

## Close out

- Run the verify-step and ` + "`sdt context reindex`" + ` after the change (see
  ` + "`context/instructions/tasks.md`" + ` for the claim vocabulary).
`

// instrCommandsIndexTemplate builds context/commands/index.md: the lookup
// surface mapping each trigger to its command file and durable instruction.
// The trigger set comes from the caller so user-created triggers stay listed.
