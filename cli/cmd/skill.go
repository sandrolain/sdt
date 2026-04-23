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
echo "hello" | sdt base64
echo "aGVsbG8=" | sdt base64 dec
echo "hello" | sdt sha256
echo "hello" | sdt hex
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
Or run: ` + "`sdt memory init --project myapp --group my-org`" + `
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
- sdt base64 / sdt base64 dec
- sdt sha256 / sdt sha512 / sdt md5
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
  go install github.com/sandrolain/sdt/cli@latest

## Quick Reference

| Goal | Command |
|---|---|
| Count tokens | ` + "`echo text | sdt tokens --model gpt-4`" + ` |
| Render prompt template | ` + "`sdt prompt render --template \"...\" --vars '{}'`" + ` |
| Validate prompt budget | ` + "`sdt prompt validate --file p.txt --max-tokens 4096`" + ` |
| Truncate to token limit | ` + "`cat doc | sdt truncate --max-tokens 2000`" + ` |
| Save memory | ` + "`sdt memory set key value --project proj`" + ` |
| Search memory | ` + "`sdt memory search query --project proj --format json`" + ` |
| Extract URLs | ` + "`echo text | sdt extract --type urls`" + ` |
| Extract code blocks | ` + "`cat response.md | sdt extract --type code-blocks`" + ` |
| Discover commands | ` + "`sdt manifest --format json`" + ` |
| Command JSON Schema | ` + "`sdt schema --command \"jwt parse\"`" + ` |
| Encode base64 | ` + "`echo data | sdt base64`" + ` |
| Hash SHA-256 | ` + "`echo data | sdt sha256`" + ` |
| Convert JSON→YAML | ` + "`cat f.json | sdt conv --from json --to yaml`" + ` |

All commands: --format json|yaml|text, --quiet, --no-color
Input: stdin | --input STRING | --file PATH
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

Examples:
  sdt skill --agent copilot
  sdt skill --agent claude --output claude-instructions.md
  sdt skill --agent generic --output AGENTS.md`,
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
	skillCmd.Flags().String("agent", "generic", "Target agent: copilot|claude|generic")
	skillCmd.Flags().String("output", "", "Output file path (default: stdout)")
	rootCmd.AddCommand(skillCmd)
}
