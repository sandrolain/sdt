---
kind: worklog
created_at: 2026-08-05T08:00:00Z
context: restructure agent instructions per user request
project: sdt_44f24890
---

# AGENTS.md block holds non-CLI instructions; instructions/ holds CLI usage only

## Changes

- `sdt.context/instructions/` now only holds CLI-usage files: `project.md` (trimmed of `## Project Configuration` and yaml block), `memory.md`, `reference.md`.
- Removed `README.md`, `commands.md`, `workflow.md`, `communication.md`, `planning.md`, `annotations.md`, `self-update.md`.
- AGENTS.md tagged `instructions` block now embeds the general agent instructions: Workflow, Planning/Work Logs/Annotations (merged planning+annotations), Communication, and a new Patterns (keep updated) section.
- Generator (`cli/cmd/agent.go`, `agentinstructions.go`): `instructionFiles()` emits 3 files; `agentBlockInstructions()` renders the full block; `--force` now also removes obsolete instruction files (`statusRemoved`).
- Constants pruned (`sdtInstrReadme`, `sdtInstrCommands`, `sdtInstrWorkflow`, `sdtInstrComm`, `sdtInstrPlanning`, `sdtInstrAnnotate`, `sdtInstrSelfUpd`).
- Tests updated; docs regenerated.

## Verification

- `go test ./...` pass (cmd coverage 87.3%)
- `golangci-lint run ./...` 0 issues
- `govulncheck ./...` no vulnerabilities
