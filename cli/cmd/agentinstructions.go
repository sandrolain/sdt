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
` + codeFence)
	return b.String()
}

const instrMemoryTemplate = `# Persistent Memory (file-based)

Memory is plain Markdown under ` + "`sdt.context/memory/`" + ` — no database, no
daemon, fully offline. It holds durable project knowledge that must survive
sessions and be readable by any agent: decisions, constraints, conventions,
concepts and references that are hard to reconstruct from code or git.

` + "`sdt memory`" + ` no longer exists. Manage memory by editing the Markdown
files directly, following this skill.

## Layout

- ` + "`sdt.context/memory/README.md`" + ` — the protocol (always follow it)
- ` + "`sdt.context/memory/index.md`" + ` — page index (` + "`[[id]]`" + ` + one-line summary)
- ` + "`sdt.context/memory/pages/<id>.md`" + ` — one durable unit of knowledge per file

## Page format

` + "`<id>`" + ` is kebab-case and must equal the filename (no ` + "`.md`" + `). Each page has
frontmatter, a ` + "`<!-- compiled_truth -->`" + ` section and an append-only timeline:

` + codeFence + `markdown
---
id: decision-auth-flow
title: Use JWT for auth
category: decision
status: active
tags: [auth, jwt]
created: "2026-09-04T00:00:00"
updated: "2026-09-04T00:00:00"
---

<!-- compiled_truth -->

<current best understanding: what was decided, alternatives, rationale, blast radius>

## Timeline

- time: 2026-09-04T00:00:00
  kind: decision
  summary: <one line>
  source: <conversation, PR, analysis>
` + codeFence + `

Categories (must be exactly one):

| category | what to write |
|---|---|
| ` + "`decision`" + ` | an established judgment and its reasoning (most common) |
| ` + "`concept`" + ` | a term / mechanism needing shared, lasting understanding |
| ` + "`project`" + ` | state and intent of a work package not readable from code |
| ` + "`person`" + ` | a person / role, preferences, collaboration conventions |
| ` + "`reference`" + ` | an external resource or object of analysis worth keeping |

Status: ` + "`active`" + ` (day-to-day view) | ` + "`draft`" + ` | ` + "`archived`" + `.

## Rules

- compiled_truth is rewritable; the timeline is **append-only**. Every truth
  rewrite appends a timeline entry (` + "`kind: decision`" + `); overturning a
  conclusion appends ` + "`kind: reversal`" + `. Never edit existing timeline entries.
- Reference other pages with ` + "`[[id]]`" + `; keep ` + "`index.md`" + ` in sync.
- Write in the user's working language; keep ids, tags and paths verbatim.
- The test for what belongs here: will this still matter in six months, and is
  it hard to reconstruct from code or git? Implementation details stay in code
  and commits.

## Discipline

- Session start: read ` + "`index.md`" + ` and the pages relevant to the task.
- When a decision, requirement or constraint settles while coding, capture it
  immediately — don't batch it to the end.
- Pure implementation with no new decision: don't write memory.
- Overturning a past conclusion: update compiled_truth and append a reversal.
- Work progress goes to ` + "`sdt.context/worklog/`" + ` and plans to
  ` + "`sdt.context/plan/`" + `, not to memory.
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

Memory is file-based (no database): ` + "`sdt.context/memory/`" + `, protocol in
` + "`sdt.context/instructions/memory.md`" + `. ` + "`sdt memory`" + ` no longer exists.

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
