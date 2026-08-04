## sdt agent guide

Generate an extended SDT skill guide

### Synopsis

Generate an extended skill guide in a dedicated directory.

Creates a multi-file skill that teaches agents how to use SDT:

  SKILL.md      — entry point with YAML frontmatter
  REFERENCE.md  — full command reference
  WORKFLOWS.md  — end-to-end workflows

Existing files are preserved unless --force is given.

Examples:
  sdt agent guide
  sdt agent guide --dir .agents/skills/sdt --force

```
sdt agent guide [flags]
```

### Options

```
      --dir string   Output directory for the guide (default ".agents/skills/sdt")
      --dry-run      Preview files without writing
      --force        Overwrite existing files
  -h, --help         help for guide
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, sections, guides)

