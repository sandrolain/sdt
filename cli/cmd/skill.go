package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// skillTemplates holds agent-specific instruction templates.
var skillTemplates = map[string]string{
	"copilot": `# SDT — Smart Developer Tools

## Overview
SDT is a CLI toolset for AI agents and developers. It provides deterministic,
composable commands for data manipulation, encoding, cryptography, templating,
persistent memory, and protocol utilities with machine-readable output.

## Usage Pattern
` + "```" + `bash
# All commands support --format json|yaml|text and --quiet
sdt <command> [subcommand] [flags]

# Input can come from stdin, --input "string", or --file path
echo "data" | sdt <command>
sdt <command> --input "data"
sdt <command> --file path/to/file
` + "```" + `

## Key Commands

### Token & Prompt Utilities
` + "```" + `bash
# Count LLM tokens (approximate, no API required)
echo "your text" | sdt tokens --model gpt-4
sdt tokens --model claude --file prompt.txt --format json

# Render a prompt template
sdt prompt render --template "You are {{.role}}." --vars '{"role":"assistant"}'
sdt prompt render --file system.txt --vars-file ctx.json --show-tokens

# Validate prompt against token budget
sdt prompt validate --file prompt.txt --max-tokens 4096 --model gpt-4 --format json

# Truncate text to token limit
cat long_doc.md | sdt truncate --max-tokens 4000
sdt truncate --max-tokens 2000 --strategy sentences --file essay.txt
sdt truncate --max-tokens 1000 --strategy sections --model claude
` + "```" + `

### Persistent Memory (per-project, offline, FTS5 search)
` + "```" + `bash
sdt memory set "key" "value" --project myapp --tags "tag1,tag2"
sdt memory get "key" --project myapp
sdt memory search "query terms" --project myapp --format json
sdt memory list --project myapp --format json
sdt memory delete "key" --project myapp
` + "```" + `

### Data Extraction
` + "```" + `bash
echo "See https://example.com and contact alice@test.com" | sdt extract --type urls
sdt extract --type emails --file response.txt
sdt extract --type code-blocks --file llm_output.md
` + "```" + `

### Encoding & Hashing
` + "```" + `bash
echo "hello" | sdt b64
echo "aGVsbG8=" | sdt b64 dec
echo "hello" | sdt sha256
echo "hello" | sdt hex
echo "hello" | sdt hmac --key secret
` + "```" + `

### JSON / YAML / Conversion
` + "```" + `bash
cat data.json | sdt json pretty
echo '{"a":1}' | sdt json valid
sdt conv --from json --to yaml --file data.json
` + "```" + `

### Template Rendering
` + "```" + `bash
echo '{"name":"World"}' | sdt template --tmpl "Hello, {{.name}}!"
sdt template --data '{"env":"prod"}' --file deploy.tmpl
` + "```" + `

### Discover All Capabilities
` + "```" + `bash
sdt manifest --format json          # full command tree (machine-readable)
sdt schema --format json            # JSON Schema for all commands
sdt schema --command "jwt parse"    # schema for a specific command
` + "```" + `

## Output Formats
All commands support ` + "`--format text|json|yaml`" + ` and ` + "`--quiet`" + `.
When stdout is not a TTY, ANSI colors are automatically disabled.

## Project Memory Config
Create ` + "`.sdt.yaml`" + ` in your project root:
` + "```" + `yaml
project: myapp
group: my-org
` + "```" + `
Or run: ` + "`sdt setup --project myapp`" + `
`,

	"claude": `<tool_instructions>
SDT (Smart Developer Tools) is a CLI toolset available as the "sdt" command.
Use it for data manipulation, encoding, cryptography, templating, persistent
memory, and LLM utilities. All output can be structured (--format json|yaml).

## Core Patterns
Input: stdin | --input "string" | --file path
Output: --format text|json|yaml (default: text)
Errors: non-zero exit code + message to stderr

## Token & Prompt Commands
- sdt tokens [--model gpt-4|claude|llama|gpt-2] < text
  Returns approximate token count (no API required).
- sdt prompt render --template "..." --vars '{"k":"v"}'
  Renders a Go text/template with JSON/YAML variables.
- sdt prompt validate --file p.txt --max-tokens 4096 --format json
  Checks a rendered prompt fits within a token budget.
- sdt truncate --max-tokens N [--strategy chars|sentences|sections]
  Truncates text to fit within a token budget.

## Memory Commands (offline, SQLite FTS5)
- sdt memory set KEY VALUE --project P [--tags t1,t2]
- sdt memory get KEY --project P
- sdt memory search QUERY --project P --format json
- sdt memory list --project P --format json
- sdt memory delete KEY --project P

## Useful Utilities
- sdt extract --type urls|emails|ips|code-blocks|json-blocks|dates < text
- sdt template --tmpl "{{.key}}" --data '{"key":"val"}'
- sdt manifest --format json  (discover all commands)
- sdt schema --command "CMD"  (JSON Schema for a command)
- sdt b64 / sdt b64 dec
- sdt sha256 / sdt sha512 / sdt md5
- sdt hmac --key secret < data
- sdt json pretty / sdt json valid
- sdt conv --from json --to yaml
- sdt jwt parse / sdt jwt claims

## Project Config (.sdt.yaml)
project: myapp
group: my-org
</tool_instructions>
`,

	"generic": `# SDT — Smart Developer Tools Agent Instructions

SDT provides composable CLI utilities for AI agents. Install with:
  go install github.com/sandrolain/sdt@latest

## Quick Reference

| Goal | Command |
|---|---|
| Count LLM tokens | ` + "`sdt tokens --model gpt-4 --input TEXT`" + ` |
| Render prompt template | ` + "`sdt prompt render --template \"...\" --vars '{}'`" + ` |
| Validate prompt budget | ` + "`sdt prompt validate --file p.txt --max-tokens 4096`" + ` |
| Truncate to token limit | ` + "`sdt truncate --max-tokens 2000 --file doc.txt`" + ` |
| Save memory | ` + "`sdt memory set key value --project proj`" + ` |
| Search memory | ` + "`sdt memory search query --project proj --format json`" + ` |
| Extract URLs | ` + "`sdt extract --type urls --file text.txt`" + ` |
| Extract code blocks | ` + "`sdt extract --type code-blocks --file response.md`" + ` |
| Discover commands | ` + "`sdt manifest --format json`" + ` |
| Command JSON Schema | ` + "`sdt schema --command \"jwt parse\"`" + ` |
| Encode base64 | ` + "`sdt b64 --input data`" + ` |
| Hash SHA-256 | ` + "`sdt sha256 --input data`" + ` |
| Convert JSON to YAML | ` + "`sdt conv --from json --to yaml --file f.json`" + ` |

All commands: --format json\|yaml\|text, --quiet, --no-color
Input: stdin \| --input STRING \| --file PATH
`,

	// "skill" produces a SKILL.md with YAML frontmatter for the open agent skills ecosystem
	// (npx skills add / .agents/skills/sdt/SKILL.md)
	"skill": `---
name: sdt
description: SDT (Smart Developer Tools) is a CLI toolset for AI agents and developers. Provides deterministic, composable commands for encoding, hashing, cryptography (HMAC, sign/verify, TLS certs), JWT, data conversion, templating, persistent memory (SQLite FTS5), LLM token counting, prompt rendering/validation, text truncation, data extraction, network utilities (DNS, port check), and more. All output is machine-readable (--format json|yaml). Pure-Go, no CGO, cross-platform.
---

# SDT — Smart Developer Tools

Install: ` + "`go install github.com/sandrolain/sdt@latest`" + `

## Core Input/Output Patterns

` + "```" + `bash
# Input sources (interchangeable on all commands)
echo "data" | sdt <cmd>                # stdin
sdt <cmd> --input "data"               # inline string
sdt <cmd> --file path/to/file          # file path

# Output format (all commands)
sdt <cmd> --format text                # default
sdt <cmd> --format json
sdt <cmd> --format yaml
sdt <cmd> --quiet                      # suppress info, only result
sdt <cmd> --no-color                   # disable ANSI
` + "```" + `

## Encoding

` + "```" + `bash
# Base64
echo "hello" | sdt b64                        # encode
echo "aGVsbG8=" | sdt b64 dec                 # decode
echo "hello" | sdt b64url                     # URL-safe encode
echo "aGVsbG8=" | sdt b64url dec              # URL-safe decode

# Base32
echo "hello" | sdt b32
echo "NBSWY3DPEB3W64TMMQ======" | sdt b32 dec

# Hex
echo "hello" | sdt hex
echo "68656c6c6f" | sdt hex dec

# URL encoding
echo "hello world" | sdt url enc
echo "hello+world" | sdt url dec
echo "a=1&b=2" | sdt url encform

# HTML encoding
echo "<b>hi</b>" | sdt html encode
echo "&lt;b&gt;hi&lt;/b&gt;" | sdt html decode
` + "```" + `

## Hashing & HMAC

` + "```" + `bash
echo "hello" | sdt md5
echo "hello" | sdt sha1
echo "hello" | sdt sha256
echo "hello" | sdt sha384
echo "hello" | sdt sha512

# HMAC (webhook signature verification, message authentication)
echo "payload" | sdt hmac --key "secret"
echo "payload" | sdt hmac --key "secret" --algo sha512
echo "payload" | sdt hmac --key "secret" --format json
# algo: sha256 (default), sha512, sha384
` + "```" + `

## Cryptography — Sign & Verify

` + "```" + `bash
# Sign data with private key (PEM)
echo "payload" | sdt sign --key private.pem
echo "payload" | sdt sign --key private.pem --algo rsa-sha512
echo "payload" | sdt sign --key ed25519.pem --algo ed25519 --format json
# algo: rsa-sha256 (default), rsa-sha512, ecdsa-sha256, ecdsa-sha512, ed25519

# Verify signature
echo "payload" | sdt verify --key public.pem --sig <base64sig>
echo "payload" | sdt verify --key public.pem --sig <base64sig> --algo ecdsa-sha256

# Generate RSA/ECDSA/Ed25519 key pairs
sdt keypair --algo rsa --bits 4096
sdt keypair --algo ecdsa --curve P-256
sdt keypair --algo ed25519
` + "```" + `

## TLS Certificates

` + "```" + `bash
# Inspect certificate from live host
sdt cert inspect --host example.com
sdt cert inspect --host example.com:8443 --format json

# Inspect local PEM file
sdt cert inspect --file cert.pem --format yaml

# Check expiry only
sdt cert expiry --host example.com
sdt cert expiry --file cert.pem --format json
` + "```" + `

## JWT

` + "```" + `bash
# Parse JWT (no verification)
echo "$TOKEN" | sdt jwt parse
echo "$TOKEN" | sdt jwt parse --format json

# Extract claims
echo "$TOKEN" | sdt jwt claims
echo "$TOKEN" | sdt jwt claims --format json

# Validate JWT (verify signature + expiry)
echo "$TOKEN" | sdt jwt valid --key public.pem
` + "```" + `

## Data Conversion

` + "```" + `bash
# Convert between formats: json, yaml, toml, msgpack, csv
sdt conv --from json --to yaml --file data.json
sdt conv --from yaml --to toml --file config.yaml
echo '{"a":1}' | sdt conv --from json --to msgpack

# Diff two files
sdt diff --file-a before.json --file-b after.json
sdt diff --file-a old.yaml --file-b new.yaml --format json
` + "```" + `

## JSON Tools

` + "```" + `bash
cat data.json | sdt json pretty
cat data.json | sdt json minify
echo '{"a":1}' | sdt json valid
` + "```" + `

## Template Rendering

` + "```" + `bash
# Go text/template with JSON data
echo '{"name":"World"}' | sdt template --tmpl "Hello, {{.name}}!"
sdt template --data '{"env":"prod"}' --file deploy.tmpl --format text

# .env files
sdt env parse --file .env --format json
sdt env get KEY --file .env
sdt env set KEY VALUE --file .env
sdt env merge --file .env --file .env.local --format json
` + "```" + `

## LLM — Token Counting, Prompt, Truncation

` + "```" + `bash
# Count approximate tokens (no API needed)
echo "your text" | sdt tokens
echo "your text" | sdt tokens --model gpt-4
sdt tokens --model claude --file prompt.txt --format json
# models: gpt-4, gpt-2, claude, llama (default: gpt-2)

# Render a prompt template
sdt prompt render --template "You are {{.role}}." --vars '{"role":"assistant"}'
sdt prompt render --file system.txt --vars-file ctx.json --show-tokens

# Validate prompt fits within token budget
sdt prompt validate --file prompt.txt --max-tokens 4096 --model gpt-4
sdt prompt validate --file prompt.txt --max-tokens 4096 --format json

# Truncate text to fit in token budget
cat long_doc.md | sdt truncate --max-tokens 4000
sdt truncate --max-tokens 2000 --strategy sentences --file essay.txt
sdt truncate --max-tokens 1000 --strategy sections --model claude
# strategy: chars (default), sentences, sections
` + "```" + `

## Persistent Memory (offline, SQLite FTS5)

Stores key-value entries per project, with full-text search.

` + "```" + `bash
# Initialize project config
sdt memory init --project myapp --group my-org

# Store and retrieve
sdt memory set "key" "value" --project myapp
sdt memory set "key" "value" --project myapp --tags "tag1,tag2"
sdt memory get "key" --project myapp
sdt memory get "key" --project myapp --format json

# Full-text search (BM25 ranking)
sdt memory search "query terms" --project myapp
sdt memory search "query terms" --project myapp --format json

# List and manage
sdt memory list --project myapp --format json
sdt memory delete "key" --project myapp
sdt memory delete --all --project myapp

# Import / export
sdt memory export --project myapp --format json
sdt memory import --project myapp --file backup.json

# Discovery
sdt memory projects
sdt memory groups
` + "```" + `

## Data Extraction

Extract structured items from unstructured text:

` + "```" + `bash
echo "Visit https://example.com or email alice@test.com" | sdt extract --type urls
echo "Visit https://example.com or email alice@test.com" | sdt extract --type emails
sdt extract --type ips --file log.txt
sdt extract --type code-blocks --file llm_output.md
sdt extract --type json-blocks --file response.txt
sdt extract --type dates --file document.txt --format json
` + "```" + `

## Network Utilities

` + "```" + `bash
# DNS lookup
sdt dns --host example.com
sdt dns --host example.com --type MX --format json
sdt dns --host example.com --type A,AAAA,MX --format json

# TCP port check
sdt port --host example.com --port 443
sdt port --host db.internal --port 5432 --timeout 2s --format json

# IP geolocation
sdt ipinfo --ip 8.8.8.8 --format json

# NS lookup
sdt nslookup --host example.com
` + "```" + `

## Unique IDs & Passwords

` + "```" + `bash
sdt uid v4               # UUID v4
sdt uid nano             # NanoID
sdt uid ks               # K-Sortable UID (KSUID)

sdt password             # secure random password
sdt password --length 32 --symbols
` + "```" + `

## TOTP (2FA)

` + "```" + `bash
sdt totp uri --account user@example.com --issuer MyApp --secret BASE32SECRET
sdt totp code --secret BASE32SECRET
sdt totp verify --secret BASE32SECRET --code 123456
sdt totp image --secret BASE32SECRET --output qr.png
` + "```" + `

## String Tools

` + "```" + `bash
echo "hello world" | sdt string uppercase
echo "HELLO WORLD" | sdt string lowercase
echo "hello world" | sdt string titlecase
echo "hello world" | sdt string replacespace --replacement _
echo "hello\nworld" | sdt string count --type lines
echo "hello\tworld" | sdt string escape
echo "hello\\tworld" | sdt string unescape
` + "```" + `

## Time

` + "```" + `bash
sdt time iso              # current time as ISO 8601
sdt time unix             # current Unix timestamp
sdt time http             # current time in HTTP date format
` + "```" + `

## Version & Manifest

` + "```" + `bash
sdt version               # build info (version, commit, date)
sdt version --format json

# Discover all commands (machine-readable)
sdt manifest --format json

# JSON Schema for a command (useful for LLM tool-calling)
sdt schema --format json
sdt schema --command "jwt parse"
sdt schema --command "memory set"
` + "```" + `

## Version Manager

` + "```" + `bash
# Bump semver strings
echo "1.2.3" | sdt vman major        # 2.0.0
echo "1.2.3" | sdt vman minor        # 1.3.0
echo "1.2.3" | sdt vman patch        # 1.2.4
echo "1.2.3" | sdt vman prerelease --pre alpha  # 1.2.3-alpha
echo "1.2.3-alpha" | sdt vman metadata --meta build.1  # 1.2.3-alpha+build.1
` + "```" + `

## Compression & File

` + "```" + `bash
cat file.txt | sdt gzip > file.txt.gz
cat file.txt.gz | sdt gunzip

sdt read --file path/to/file
sdt write --file output.txt --input "content"

sdt bytes --size 32                  # 32 random bytes (base64)
sdt bytes --size 32 --format hex     # as hex
` + "```" + `

## HTTP, Crawling, Regex, QR

` + "```" + `bash
# HTTP client (headers, method, body, timeout)
sdt http --url https://api.example.com
sdt http --url https://api.example.com --method POST --body '{"ok":true}' --format json

# Crawl site and export markdown pages
sdt crawldown https://example.com --depth 2 --output ./site-md

# Regular expressions
echo "a1 b2" | sdt regexp --pattern "[a-z][0-9]"
echo "foo-123" | sdt regexp replace --pattern "[0-9]+" --replace "X"

# QR code
sdt qrcode --text "https://example.com" --output qrcode.png
sdt qrcode read --file qrcode.png
` + "```" + `

## Password Hashing (bcrypt)

` + "```" + `bash
echo "my-password" | sdt bcrypt
sdt bcrypt verify --password "my-password" --hash "$2a$..."
` + "```" + `

## Project Bootstrap, Config, Docs

` + "```" + `bash
# Project bootstrap for agents
sdt setup --project myapp --group my-org --agent all
sdt setup --project myapp --agent skill --force

# Generate instructions/skills on demand
sdt skill --agent generic --output AGENTS.md
sdt skill --agent skill --output .agents/skills/sdt/SKILL.md

# Read/write config values
sdt config get api.base_url
sdt config set api.base_url https://example.com

# Generate CLI docs and shell completions
sdt docs --output ./docs
sdt completion zsh > ~/.zsh/completions/_sdt

# Command help
sdt help
sdt help memory
` + "```" + `

## Project Config (.sdt.yaml)

Create with ` + "`sdt setup --project myapp`" + ` or manually:

` + "```" + `yaml
project: myapp
group: my-org
` + "```" + `

SDT searches for ` + "`.sdt.yaml`" + ` by walking up from the current directory (like ` + "`.git`" + `).
`,
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Generate agent skill/instruction files for using SDT",
	Long: `Generate instruction files that teach AI agents how to use SDT.

Supported agents:
  copilot   GitHub Copilot / VS Code agent instructions (Markdown)
  claude    Claude / Anthropic agent tool instructions (XML-tagged Markdown)
  generic   Generic agent-agnostic instructions (Markdown table)
  skill     SKILL.md with YAML frontmatter (.agents/skills ecosystem)

Examples:
  sdt skill --agent copilot
  sdt skill --agent claude --output claude-instructions.md
  sdt skill --agent generic --output AGENTS.md
  sdt skill --agent skill --output .agents/skills/sdt/SKILL.md`,
	Run: func(cmd *cobra.Command, args []string) {
		agent := getStringFlag(cmd, "agent", false)
		if agent == "" {
			agent = "generic"
		}
		outputPath := getStringFlag(cmd, "output", false)

		content, ok := skillTemplates[agent]
		if !ok {
			var names []string
			for k := range skillTemplates {
				names = append(names, k)
			}
			exitWithError(cmd, fmt.Errorf("unknown agent %q; supported: %s", agent, strings.Join(names, ", ")))
			return
		}

		if outputPath != "" {
			exitWithError(cmd, os.WriteFile(outputPath, []byte(content), 0o600)) //#nosec G306 -- user-chosen output file
			outputString(cmd, fmt.Sprintf("written to %s", outputPath))
			return
		}
		outputString(cmd, content)
	},
}

func init() {
	skillCmd.Flags().String("agent", "generic", "Target agent: copilot|claude|generic|skill")
	skillCmd.Flags().String("output", "", "Output file path (default: stdout)")
	rootCmd.AddCommand(skillCmd)
}
