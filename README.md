# sdt

[![CI](https://github.com/sandrolain/sdt/workflows/CI/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/ci.yml)
[![Security](https://github.com/sandrolain/sdt/workflows/Security/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sandrolain/sdt)](https://goreportcard.com/report/github.com/sandrolain/sdt)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Smart Developer Tools** — a composable CLI toolset for AI agents and developers.

<img src="./sdt-gopher.png" height="480" />

## What is sdt

`sdt` is a command-line toolset built for AI agents and developers who work with
them. It provides deterministic, composable commands for encoding, hashing and
cryptography, JWT handling, data conversion and templating, file-based
persistent memory, text and prompt utilities, and network diagnostics. Every
command is pipe-friendly and produces machine-readable output, so agents can
build reliable automation on top of it.

## Features

- **Machine-readable output** — `--format json|yaml|text` on every command; ANSI suppressed automatically when stdout is not a TTY
- **Pipe-friendly** — reads stdin, writes stdout, errors to stderr; composable with shell pipes
- **File-based knowledge** — durable project knowledge as versioned Markdown under `sdt.context/` (architecture, decisions/ADRs, plans, worklogs, notes, tasks) with a generated index, fully offline, no database
- **AI-agent tooling** — manifest discovery, command schemas, generated per-command docs, project work files and task lists
- **Context management** — structured knowledge base with typed documents, a generated index (`sdt context reindex`), linting (`sdt context lint`), and agent instruction files in `sdt.context/instructions/`
- **Agent instructions** — per-project conventions, CLI reference, and workflow guides generated and maintained under `sdt.context/instructions/`
- **Zero CGO** — pure-Go build, no C toolchain required
- **Cross-platform** — Linux, macOS, Windows

## Installation

```bash
go install github.com/sandrolain/sdt@latest
```

Build from source:

```bash
git clone https://github.com/sandrolain/sdt
cd sdt
go build -o bin/sdt ./cli
```

## Documentation

- `docs/` — per-command reference (regenerate with `sdt docs`)
- `sdt.context/instructions/cli.md` — curated CLI usage and examples for agents
- `sdt.context/instructions/reference.md` — SDT command reference overview
- `AGENTS.md` — conventions for agents working on this repository
- [CONTRIBUTING.md](./CONTRIBUTING.md) — contribution guidelines

## Development

```bash
# Run tests (≥80% coverage required)
go test ./...

# Lint
golangci-lint run ./...

# Vulnerability check
govulncheck ./...
```

## Context & Knowledge

Project knowledge lives under `sdt.context/` as versioned Markdown files:

- `architecture/` — living architecture docs (essential tier)
- `decisions/` — Architecture Decision Records (ADRs), append-only
- `plan/` — development plans
- `tasks/` — per-phase task files
- `worklog/` — final reports, append-only
- `notes/` — free-form annotations
- `questions/` — open questions with provenance
- `instructions/` — agent instruction files and templates
- `index.md` — generated knowledge index (entry point)

Use `sdt context reindex` to regenerate the index and `sdt context lint` to
validate document structure. Full reference: `sdt context --help`.

## License

[MIT License](LICENSE) — Copyright (c) 2025 Sandro Lain
