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

Discover all capabilities:
`)
	b.WriteString(codeFence + `
sdt manifest --format json
sdt schema --command "<command>"
` + codeFence)
	return b.String()
}

const instrMemoryTemplate = `# Persistent Memory

Use ` + "`sdt memory`" + ` to store facts that must survive across sessions:
decisions, constraints, conventions and known-good choices.

` + codeFence + `
sdt memory set <key> <value> [--tags a,b]   # remember a fact
sdt memory get <key>                         # retrieve a fact
sdt memory search <query> --format json      # full-text search (BM25)
sdt memory list --format json                # list all entries
sdt memory delete <key>                      # forget a fact
` + codeFence + `

Rules:
- Use stable, descriptive keys (e.g. ` + "`arch:database`" + `, ` + "`decision:auth`" + `).
- Prefer durable project facts over transient data.
- Store work progress in ` + "`sdt.context/worklog/`" + `, not in memory.
- Review everything at session start: ` + "`sdt memory list --format json`" + `.
`

const instrReferenceTemplate = `# SDT — Command Reference

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
sdt agent init --project myapp --yes     # bootstrap AGENTS.md + instructions
sdt manifest --format json               # full command tree
sdt schema --command "memory set"        # JSON Schema for a command
sdt config show                          # resolved project configuration
sdt version --format json                # build info
` + codeFence + `
`
