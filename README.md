# sdt

[![CI](https://github.com/sandrolain/sdt/workflows/CI/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/ci.yml)
[![Security](https://github.com/sandrolain/sdt/workflows/Security/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sandrolain/sdt)](https://goreportcard.com/report/github.com/sandrolain/sdt)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Smart Developer Tools** — A composable CLI toolset for AI agents and developers.

> ⚠️ **Work in Progress**: Active development. See [AGENTS.md](./AGENTS.md) for agent-oriented instructions.

## Features

- **Machine-readable output** — `--format json|yaml|text` on every command; ANSI suppressed automatically when stdout is not a TTY
- **Pipe-friendly** — reads stdin, writes stdout, errors to stderr; composable with shell pipes
- **Persistent memory** — key-value store with full-text search (SQLite FTS5, pure-Go, offline)
- **AI-agent tooling** — manifest discovery, text extraction, template rendering, env management
- **Zero CGO** — pure-Go build, no C toolchain required
- **Cross-platform** — Linux, macOS, Windows

## Quick Start

### Installation

```bash
go install github.com/sandrolain/sdt/cli@latest
```

```bash
# Build from source
git clone https://github.com/sandrolain/sdt
cd sdt
go build -o bin/sdt ./cli
```

### Global Flags

All commands support these persistent flags:

| Flag | Description |
|---|---|
| `--format text\|json\|yaml` | Output format (default: `text`) |
| `--quiet` | Suppress informational messages |
| `--no-color` | Disable ANSI color codes |
| `--input <string>` | Provide input as a flag |
| `--file <path>` | Read input from file |
| `--inb64 <base64>` | Provide input as Base64 |

### Project Configuration

For project-scoped commands (e.g. `memory`), create `.sdt.yaml` in the repository root:

```yaml
project: myapp
group: acme-platform
```

Or run: `sdt memory init --project myapp --group acme-platform`

SDT searches for `.sdt.yaml` by walking up from the current directory.

---

## Commands

### Agent Tooling

| Command | Description |
|---|---|
| `manifest` | Emit a JSON/YAML manifest of all commands (auto-discovery) |
| `memory set` | Store a key-value entry in persistent memory |
| `memory get` | Retrieve a value by key |
| `memory list` | List entries for a project |
| `memory search` | Full-text search (FTS5) across entries |
| `memory delete` | Delete an entry or all entries for a project |
| `memory projects` | List all known projects |
| `memory groups` | List all known groups |
| `memory export` | Export memory as JSON |
| `memory import` | Import memory from JSON |
| `memory init` | Create `.sdt.yaml` in the current directory |
| `setup` | Scaffold `.sdt.yaml` and agent instruction files in the current directory |
| `skill` | Generate agent instruction templates (`copilot`, `claude`, `generic`, `skill`) |
| `extract` | Extract URLs, emails, IPs, JSON blocks, code blocks from text |
| `template` | Render Go templates with JSON/YAML data |
| `env parse` | Parse `.env` file as JSON |
| `env get` | Get a value from a `.env` file |
| `env set` | Set a value in a `.env` file |
| `env merge` | Merge multiple `.env` files |
| `diff` | Unified or JSON-patch diff of two inputs |

### Encoding & Decoding

| Command | Description |
|---|---|
| `b32` / `b32 dec` | Base32 encode/decode |
| `b64` / `b64 dec` | Base64 encode/decode |
| `b64url` / `b64url dec` | Base64 URL-safe encode/decode |
| `hex` / `hex dec` | Hexadecimal encode/decode |
| `url enc` / `url dec` | URL encode/decode |
| `html encode` / `html decode` | HTML entity encode/decode |
| `bytes` | Generate random bytes |
| `bytes dec` | Decimal encoding of bytes |

### Hashing & Cryptography

| Command | Description |
|---|---|
| `md5` / `sha1` / `sha256` / `sha384` / `sha512` | Hash functions |
| `bcrypt` / `bcrypt verify` | Bcrypt hash and verification |
| `keypair` | Generate RSA/Ed25519/ECDSA key pairs |
| `hmac` | Compute HMAC (`sha256`, `sha384`, `sha512`) |
| `sign` / `verify` | Sign and verify input (RSA, ECDSA, Ed25519) |
| `cert inspect` / `cert expiry` | Inspect TLS/X.509 certificates and expiration |

### JWT & Tokens

| Command | Description |
|---|---|
| `jwt parse` | Parse JWT token (header, claims, signature) |
| `jwt claims` | Extract JWT claims |
| `jwt valid` | Validate JWT signature |

### Data Conversion

| Command | Description |
|---|---|
| `conv --in X --out Y` | Convert between json, yaml, toml, csv, msgpack |
| `json pretty` | Pretty-print JSON |
| `json minify` | Minify JSON |
| `json valid` | Validate JSON |

### UUID & IDs

| Command | Description |
|---|---|
| `uid v4` | UUID v4 |
| `uid nano` | Nano ID |
| `uid ks` | KSUID |

### Time & Date

| Command | Description |
|---|---|
| `time unix` | Current Unix timestamp |
| `time iso` | ISO 8601 timestamp |
| `time http` | HTTP date format |

### String Operations

| Command | Description |
|---|---|
| `string uppercase` / `lowercase` / `titlecase` | Case conversion |
| `string count` | Count characters/words/lines |
| `string escape` / `unescape` | Escape/unescape special characters |
| `string replacespace` | Replace spaces with a separator |
| `regexp` / `regexp replace` | Regex match/replace |

### Network

| Command | Description |
|---|---|
| `http` | HTTP client (GET/POST/PUT/…) |
| `ipinfo` | IP geolocation (JSON output) |
| `nslookup` | DNS lookup |
| `dns` | DNS lookup with typed records (`A`, `AAAA`, `MX`, `TXT`, `CNAME`, `NS`, `PTR`) |
| `port` | TCP port availability and latency check |

### Other

| Command | Description |
|---|---|
| `gzip` / `gunzip` | Compress/decompress |
| `password` | Generate secure password |
| `qrcode` / `qrcode read` | QR code generation/reading |
| `totp` | TOTP URI, code, verify |
| `vman` | Semantic version manipulation |
| `config get` / `config set` | Read/write `sdt.yaml` config |
| `version` | Print build version info |

---

## Usage Examples

### AI Agent: load context

```bash
CONTEXT=$(sdt memory search "project architecture" --project myapp --format json)
```

### AI Agent: save a decision

```bash
sdt memory set "db-decision" "Using PostgreSQL for relational data" \
  --project myapp --tags "database,architecture"
```

### Render a prompt template

```bash
echo '{"user":"Alice","task":"summarise"}' | \
  sdt template --tmpl "You are helping {{.user}} to {{.task}} the following:"
```

### Extract URLs from LLM response

```bash
cat llm_response.txt | sdt extract --type urls
```

### Diff two JSON files

```bash
sdt diff --a old.json --b new.json --format json-patch
```

### Convert YAML config to JSON

```bash
cat config.yaml | sdt conv --in yaml --out json
```

### Hash pipeline

```bash
echo "password" | sdt sha256 | sdt hex
```

---

## Development

```bash
# Run tests (≥80% coverage required)
go test ./...

# Lint
golangci-lint run ./...

# Vulnerability check
govulncheck ./...

# Build
go build -o bin/sdt ./cli
```

See [AGENTS.md](./AGENTS.md) for agent conventions and [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidelines.

---

## License

[MIT License](LICENSE) — Copyright (c) 2025 Sandro Lain
