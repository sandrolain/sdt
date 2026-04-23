## sdt skill

Generate agent skill/instruction files for using SDT

### Synopsis

Generate instruction files that teach AI agents how to use SDT.

Supported agents:
  copilot   GitHub Copilot / VS Code agent instructions (Markdown)
  claude    Claude / Anthropic agent tool instructions (XML-tagged Markdown)
  generic   Generic agent-agnostic instructions (Markdown table)
  skill     SKILL.md with YAML frontmatter (.agents/skills ecosystem)

Examples:
  sdt skill --agent copilot
  sdt skill --agent claude --output claude-instructions.md
  sdt skill --agent generic --output AGENTS.md
  sdt skill --agent skill --output .agents/skills/sdt/SKILL.md

```
sdt skill [flags]
```

### Options

```
      --agent string    Target agent: copilot|claude|generic|skill (default "generic")
  -h, --help            help for skill
      --output string   Output file path (default: stdout)
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools

