## sdt setup

Scaffold agent files for the current project

### Synopsis

Create agent instruction files and .sdt.yaml in the current directory.

By default (--agent all), the following files are created:

  .sdt.yaml                       — project identity for sdt memory
  AGENTS.md                       — generic agent instructions
  .agents/skills/sdt/SKILL.md     — open agent skills ecosystem

Use --agent to create specific files:
  --agent generic                 AGENTS.md only
  --agent skill                   .agents/skills/sdt/SKILL.md only
  --agent copilot                 .github/copilot-instructions.md
  --agent claude                  CLAUDE.md
  --agent copilot,claude          multiple agents (comma-separated)

Use --dry-run to preview without writing anything.
Use --force to overwrite existing files.

Examples:
  sdt setup --project myapp
  sdt setup --project myapp --group platform --agent copilot
  sdt setup --project myapp --agent all --force
  sdt setup --project myapp --dry-run

```
sdt setup [flags]
```

### Options

```
      --agent string     Agent type(s): copilot|claude|generic|skill|all (comma-separated) (default "all")
      --dry-run          Preview files without writing
      --force            Overwrite existing files
      --group string     Group/team name for .sdt.yaml (optional)
  -h, --help             help for setup
      --project string   Project name for .sdt.yaml (optional)
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

