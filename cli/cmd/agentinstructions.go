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

Project-scoped commands (e.g. ` + "`sdt memory`" + `) resolve project/group from:

1. ` + "`--project`" + ` / ` + "`--group`" + ` flags
2. ` + "`.sdt.yaml`" + ` found by walking up from the current directory (like ` + "`.git`" + `)
3. otherwise a descriptive error (no implicit fallback)

Without explicit flags the default identity is ` + "`<dirname>_<short-path-hash>`" + `.

## Discovering capabilities

` + codeFence + `
sdt manifest --format json
sdt schema --command "<command>"
` + codeFence)
	return b.String()
}

const instrMemoryTemplate = `# Persistent Memory

Use ` + "`sdt memory`" + ` to store facts that must survive across sessions:
decisions, constraints, conventions and known-good choices. Storage is a local
SQLite database (` + "`~/.sdt/memory.sqlite`" + `, FTS5 full-text search); fully
offline, no external services.

` + codeFence + `
sdt memory set <key> <value> [--tags a,b]   # remember a fact
sdt memory get <key>                         # retrieve a fact
sdt memory search <query> --format json      # full-text search (BM25)
sdt memory list --format json                # list all entries
sdt memory delete <key>                      # forget a fact
sdt memory export                            # export all entries
sdt memory import --file backup.json         # import entries
sdt memory projects                          # list known projects
sdt memory groups                            # list known groups
` + codeFence + `

Identity:
- Entries are scoped by project. Project/group come from ` + "`--project`" + ` /
  ` + "`--group`" + ` flags or from ` + "`.sdt.yaml`" + ` (found walking up from the
  current directory).
- Without an identity the commands fail with a descriptive error; ` + "`memory projects`" + ` /
  ` + "`memory groups`" + ` are global and need no project.

Rules:
- Use stable, descriptive keys (e.g. ` + "`arch:database`" + `, ` + "`decision:auth`" + `).
- Prefer durable project facts over transient data.
- Store work progress in ` + "`sdt.context/worklog/`" + `, not in memory.
- Review everything at session start: ` + "`sdt memory list --format json`" + `.
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

## Memory

` + "`sdt memory`" + ` is the persistent, offline key-value store. See
` + "`sdt.context/instructions/memory.md`" + ` for full usage.

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

- ` + "`sdt memory set|get|search|list|delete|export|import`" + ` — persistent memory (see memory.md)
- ` + "`sdt context new|path|list|task|docs`" + ` — plan/worklog/notes, task list, generated docs
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
sdt memory set "arch:database" "PostgreSQL" --project myapp --tags "database,architecture"
sdt memory search "project architecture" --project myapp --format json
sdt totp code --secret BASE32SECRET
sdt dns --host example.com --type A --format json
` + codeFence + `
`
