package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

const agentGuideDefaultDir = ".agents/skills/sdt"

// FileResult is the outcome of creating or updating one file.
type FileResult struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// guideFiles maps file name -> template content, in creation order.
func guideFiles() map[string]string {
	return map[string]string{
		guideFileSkill:     agentGuideSkillTemplate,
		guideFileReference: agentGuideReferenceTemplate,
		guideFileWorkflows: agentGuideWorkflowsTemplate,
	}
}

// writeGuideFiles creates the extended skill guide files in dir. It is
// non-destructive: existing files are preserved unless force is set.
func writeGuideFiles(dir string, force, dryRun bool) []FileResult {
	var results []FileResult
	for name, content := range guideFiles() {
		path := filepath.Join(dir, name)
		res := FileResult{Path: path}
		if dryRun {
			res.Status = statusDryRun
			results = append(results, res)
			continue
		}
		existing := false
		if _, err := os.Stat(path); err == nil {
			existing = true
			if !force {
				res.Status = statusSkipped
				res.Reason = "file already exists (use --force to overwrite)"
				results = append(results, res)
				continue
			}
		}
		if err := os.MkdirAll(dir, 0o750); err != nil { //#nosec G301
			res.Status = statusError
			res.Reason = err.Error()
			results = append(results, res)
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G306 -- user-chosen output
			res.Status = statusError
			res.Reason = err.Error()
			results = append(results, res)
			continue
		}
		if existing {
			res.Status = statusUpdated
		} else {
			res.Status = statusCreated
		}
		results = append(results, res)
	}
	return results
}

func outputGuideResults(cmd *cobra.Command, results []FileResult) {
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

var agentGuideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Generate an extended SDT skill guide",
	Long: `Generate an extended skill guide in a dedicated directory.

Creates a multi-file skill that teaches agents how to use SDT:

  SKILL.md      — entry point with YAML frontmatter
  REFERENCE.md  — full command reference
  WORKFLOWS.md  — end-to-end workflows

Existing files are preserved unless --force is given.

Examples:
  sdt agent guide
  sdt agent guide --dir .agents/skills/sdt --force`,
	Run: func(cmd *cobra.Command, args []string) {
		dir := getStringFlag(cmd, "dir", false)
		if dir == "" {
			dir = agentGuideDefaultDir
		}
		force := getBoolFlag(cmd, "force", false)
		dryRun := getBoolFlag(cmd, "dry-run", false)

		outputGuideResults(cmd, writeGuideFiles(dir, force, dryRun))
	},
}

func init() {
	agentGuideCmd.Flags().String("dir", agentGuideDefaultDir, "Output directory for the guide")
	agentGuideCmd.Flags().Bool("force", false, "Overwrite existing files")
	agentGuideCmd.Flags().Bool("dry-run", false, "Preview files without writing")
}

// ── templates ──────────────────────────────────────────────────────────────────

const agentGuideSkillTemplate = `---
name: sdt
description: SDT (Smart Developer Tools) is a CLI toolset for AI agents and developers. Provides deterministic, composable commands for encoding, hashing, cryptography (HMAC, sign/verify, TLS certs), JWT, data conversion, templating, persistent memory (SQLite FTS5), LLM token counting, prompt rendering/validation, text truncation, data extraction, network utilities (DNS, port check), and more. All output is machine-readable (--format json|yaml). Pure-Go, no CGO, cross-platform.
---

# SDT — Smart Developer Tools

SDT is a composable CLI for AI agents. Every command reads from stdin and writes
to stdout; output is machine-readable and deterministic.

## Core Patterns

` + codeFence + `
echo "data" | sdt <cmd>                 # stdin
sdt <cmd> --input "data"                # inline string
sdt <cmd> --file path                   # file path
sdt <cmd> --format json|yaml|text       # output format
sdt <cmd> --quiet                       # only the result, no info
` + codeFence + `

Errors go to stderr with a non-zero exit code. ANSI colors are disabled when
stdout is not a TTY.

## Quick Reference

| Goal | Command |
|---|---|
| Discover all commands | ` + "`sdt manifest --format json`" + ` |
| Command JSON Schema | ` + "`sdt schema --command \"<cmd>\"`" + ` |
| Count LLM tokens | ` + "`sdt tokens --model gpt-4 --input \"...\"`" + ` |
| Render prompt template | ` + "`sdt prompt render --template \"...\" --vars '{}'`" + ` |
| Validate prompt budget | ` + "`sdt prompt validate --file p.txt --max-tokens 4096`" + ` |
| Save memory | ` + "`sdt memory set key value --project p`" + ` |
| Search memory | ` + "`sdt memory search query --project p`" + ` |
| Extract URLs/emails/IPs | ` + "`sdt extract --type urls --file f.txt`" + ` |
| Encode base64 | ` + "`sdt b64 --input data`" + ` |
| Hash SHA-256 | ` + "`sdt sha256 --input data`" + ` |
| Convert JSON to YAML | ` + "`sdt conv --from json --to yaml --file f.json`" + ` |
| Verify webhook HMAC | ` + "`sdt hmac --key secret`" + ` |
| Generate AGENTS.md | ` + "`sdt agent init --project myapp`" + ` |
| Generate this guide | ` + "`sdt agent guide`" + ` |

## Communication (default)

- Code responses: terse caveman (ultra). Drop articles/filler, fragments OK,
  technical terms exact, code unchanged. User-requested docs: normal.
- Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase
  after type. Body only when "why" unclear. No period on subject.
- ` + "`sdt.context/`" + ` files and docs: concise technical language. Cut
  fluff, keep meaning and readability.

## Reference

See REFERENCE.md for the full command-by-command reference, and WORKFLOWS.md
for end-to-end workflows (project bootstrap, memory management, planning and
log keeping).
`

const agentGuideReferenceTemplate = `# SDT — Command Reference

SDT (Smart Developer Tools) is a pure-Go, offline-first CLI for AI agents.

## Input / Output conventions

- Input: stdin | ` + "`--input \"string\"`" + ` | ` + "`--file path`" + ` | ` + "`--inb64`" + `
- Output: ` + "`--format text|json|yaml`" + ` (default text)
- ` + "`--quiet`" + ` suppresses informational output
- ` + "`--no-color`" + ` disables ANSI
- Errors: message to stderr + non-zero exit code

## Project configuration

` + "`.sdt.yaml`" + ` holds project identity and is found by walking up from the
current directory:

` + codeFence + `yaml
project: myapp_7f2b39e1
group: platform
` + codeFence + `

Create it with ` + "`sdt agent init --project myapp --group platform`" + `.

## Encoding

` + codeFence + `
echo "hello" | sdt b64                   # base64 encode
echo "aGVsbG8=" | sdt b64 dec            # base64 decode
echo "hello" | sdt b64url                # url-safe base64
echo "hello" | sdt b32                   # base32 encode
echo "NBSWY3DPEB3W64TMMQ======" | sdt b32 dec
echo "hello" | sdt hex                   # hex encode
echo "68656c6c6f" | sdt hex dec
echo "hello world" | sdt url enc         # percent-encoding
echo "hello world" | sdt html encode
` + codeFence + `

## Hashing & HMAC

` + codeFence + `
echo "hello" | sdt md5
echo "hello" | sdt sha1
echo "hello" | sdt sha256
echo "hello" | sdt sha512
echo "payload" | sdt hmac --key "secret"
echo "payload" | sdt hmac --key "secret" --algo sha512 --format json
echo "my-password" | sdt bcrypt
sdt bcrypt verify --password "my-password" --hash "$2a$..."
` + codeFence + `

## Cryptography — sign/verify, certs, keypairs

` + codeFence + `
echo "payload" | sdt sign --key private.pem
echo "payload" | sdt verify --key public.pem --sig <base64sig>
sdt keypair --algo rsa --bits 4096
sdt keypair --algo ed25519
sdt cert inspect --host example.com --format json
sdt cert expiry --host example.com
` + codeFence + `

## JWT

` + codeFence + `
echo "$TOKEN" | sdt jwt parse --format json
echo "$TOKEN" | sdt jwt claims
echo "$TOKEN" | sdt jwt valid --key public.pem
` + codeFence + `

## Data conversion & JSON

` + codeFence + `
sdt conv --from json --to yaml --file data.json
echo '{"a":1}' | sdt conv --from json --to msgpack
sdt diff --file-a before.json --file-b after.json --format json
cat data.json | sdt json pretty
cat data.json | sdt json minify
echo '{"a":1}' | sdt json valid
` + codeFence + `

## Templating & env

` + codeFence + `
echo '{"name":"World"}' | sdt template --tmpl "Hello, {{.name}}!"
sdt template --data '{"env":"prod"}' --file deploy.tmpl
sdt env parse --file .env --format json
sdt env get KEY --file .env
sdt env set KEY VALUE --file .env
sdt env merge --file .env --file .env.local
` + codeFence + `

## LLM utilities — tokens, prompt, truncate

` + codeFence + `
echo "your text" | sdt tokens --model gpt-4
sdt tokens --model claude --file prompt.txt --format json
sdt prompt render --template "You are {{.role}}." --vars '{"role":"assistant"}'
sdt prompt validate --file prompt.txt --max-tokens 4096 --model gpt-4
cat long.md | sdt truncate --max-tokens 4000
sdt truncate --max-tokens 2000 --strategy sentences --file essay.txt
` + codeFence + `

## Persistent memory (offline, SQLite FTS5)

` + codeFence + `
sdt memory init --project myapp --group my-org
sdt memory set "key" "value" --project myapp --tags "tag1,tag2"
sdt memory get "key" --project myapp --format json
sdt memory search "query terms" --project myapp
sdt memory list --project myapp --format json
sdt memory delete "key" --project myapp
sdt memory export --project myapp
sdt memory import --project myapp --file backup.json
sdt memory projects
sdt memory groups
` + codeFence + `

## Data extraction

` + codeFence + `
echo "Visit https://example.com or email alice@test.com" | sdt extract --type urls
echo "..." | sdt extract --type emails
sdt extract --type ips --file log.txt
sdt extract --type code-blocks --file llm_output.md
sdt extract --type json-blocks --file response.txt
sdt extract --type dates --file document.txt --format json
` + codeFence + `

## Network

` + codeFence + `
sdt dns --host example.com --type A,AAAA,MX --format json
sdt port --host example.com --port 443
sdt ipinfo --ip 8.8.8.8 --format json
sdt nslookup --host example.com
sdt http --url https://api.example.com
sdt http --url https://api.example.com --method POST --body '{"ok":true}'
sdt crawldown https://example.com --depth 2 --output ./site-md
` + codeFence + `

## IDs, passwords, TOTP

` + codeFence + `
sdt uid v4                 # UUID v4
sdt uid nano               # NanoID
sdt uid ks                 # KSUID
sdt password
sdt password --length 32 --symbols
sdt totp uri --account user@example.com --issuer MyApp --secret BASE32SECRET
sdt totp code --secret BASE32SECRET
sdt totp verify --secret BASE32SECRET --code 123456
sdt totp image --secret BASE32SECRET --output qr.png
` + codeFence + `

## String, time, version, file

` + codeFence + `
echo "hello world" | sdt string uppercase
echo "hello world" | sdt string titlecase
echo "hello\nworld" | sdt string count --type lines
echo "a1 b2" | sdt regexp --pattern "[a-z][0-9]"
sdt time iso
sdt time unix
echo "1.2.3" | sdt vman minor
echo "1.2.3" | sdt vman prerelease --pre alpha
cat file.txt | sdt gzip > file.txt.gz
sdt read --file path/to/file
sdt write --file output.txt --input "content"
sdt bytes --size 32 --format hex
sdt qrcode --text "https://example.com" --output qrcode.png
` + codeFence + `

## Agent instructions & discoverability

` + codeFence + `
sdt agent init --project myapp --yes     # bootstrap AGENTS.md + .sdt.yaml + sdt/ + skill
sdt agent section list                   # list tagged AGENTS.md sections
sdt agent section update <name>          # update one section
sdt agent guide                           # extended skill guide
sdt skill --agent generic                # instruction file for an agent
sdt config show                          # resolved project configuration
sdt manifest --format json               # full command tree
sdt schema --command "memory set"        # JSON Schema for a command
sdt version --format json                # build info
` + codeFence + `
`

const agentGuideWorkflowsTemplate = `# SDT — Workflows

## 1. Bootstrap a project with SDT

` + codeFence + `
# 1. Bootstrap everything: .sdt.yaml + AGENTS.md + sdt/ dirs + skill guide
sdt agent init --project myapp --group platform --yes

# 2. Fill in the real build/test commands for the project
sdt agent section update commands --file commands.md

# 3. Verify everything
sdt config show --format json
sdt manifest --format json
` + codeFence + `

## 2. Managing persistent memory

Use memory for facts that must survive across sessions (decisions, constraints,
conventions). Memory is per-project and searched with FTS5.

` + codeFence + `
# Remember a decision with tags
sdt memory set "arch:database" "PostgreSQL for relational data" --tags "architecture,database"

# Remember a constraint
sdt memory set "convention:errors" "all errors go to stderr with exit code 1"

# Retrieve before acting
sdt memory get "arch:database"
sdt memory search "database constraint" --format json

# Review everything at session start
sdt memory list --format json
` + codeFence + `

## 3. Planning and logging work in sdt.context/

Keep planning and logs under the project's ` + "`sdt.context/`" + ` directory. Use
date/time prefixes so files stay sortable and history is preserved.

` + codeFence + `
sdt.context/plan/<YYYY-MM-DD>-<slug>.md              # plan before starting work
sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md      # ordered log of completed work
sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md        # free-form annotations
` + codeFence + `

Use ` + "`sdt.context/tmp/`" + ` for temporary or scratch files. Never write or
execute temporary files outside the project (e.g. do not use ` + "`/tmp`" + `).

Every file starts with YAML frontmatter:

` + codeFence + `yaml
---
kind: worklog      # plan | worklog | notes
created_at: 2026-08-02T10:00:00Z
context: what triggered this entry
project: myapp
---
` + codeFence + `

### Suggested loop

1. Write a plan in ` + "`sdt.context/plan/`" + `.
2. Search memory and read AGENTS.md.
3. Implement the smallest change.
4. Verify with the project's build/test/lint commands.
5. Append a ` + "`sdt.context/worklog/`" + ` entry.
6. Store durable decisions in memory.
7. Update AGENTS.md if conventions changed.

## 4. Updating AGENTS.md safely

AGENTS.md is organized in tagged sections:

` + codeFence + `
<!-- sdt:begin:NAME -->
...content...
<!-- sdt:end:NAME -->
` + codeFence + `

Update only what changed, never rewrite the whole file:

` + codeFence + `
sdt agent section list
sdt agent section show workflow
sdt agent section update commands --file new-commands.md
sdt agent section set notes --input "decision: use feature flags"
sdt agent section remove deprecated
` + codeFence + `

## 5. Inspecting an unknown codebase

` + codeFence + `
# Discover what sdt can do
sdt manifest --format json

# Schema for a specific command (LLM tool-calling)
sdt schema --command "jwt parse"

# Token budget before sending a large prompt
sdt tokens --model claude --file big.txt

# Extract structure from LLM output
sdt extract --type json-blocks --file llm_output.md
sdt conv --from yaml --to json --file config.yaml
` + codeFence + `

## 6. Communication defaults

Code work: terse caveman ultra — drop articles/filler/pleasantries, fragments
OK, short synonyms, technical terms exact, code unchanged. Pattern:
` + "[thing] [action] [reason]. [next step]." + `
Code only; user-requested docs written normal (concise).

Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

` + "`sdt.context/`" + ` files and docs: concise technical language. Cut
fluff, keep meaning and readability.
`
