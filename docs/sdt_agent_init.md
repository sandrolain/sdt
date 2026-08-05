## sdt agent init

Bootstrap an SDT-managed project for AI agents

### Synopsis

Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                           project identity (project/group)
  AGENTS.md                           single tagged block of general instructions
  sdt.context/plan|worklog|notes|tmp  working directories
  sdt.context/instructions/           CLI usage files (project, memory, reference)
  .gitignore                          ignores sdt.context/tmp (git repos only)

The command is idempotent and non-destructive: a second run fills in missing
content and never overwrites or removes existing files. Use --force to refresh
generated content and remove obsolete instruction files.

When the current directory is inside a git repository, .gitignore is created (or
updated) with an entry ignoring the sdt.context/tmp working directory.

Values not provided via flags are prompted interactively with sensible defaults.
Use --yes to accept defaults without prompting (CI/non-interactive).

Examples:
  sdt agent init
  sdt agent init --project myapp
  sdt agent init --project myapp --group platform --yes

```
sdt agent init [flags]
```

### Options

```
      --force            Refresh generated template content
      --group string     Group name
  -h, --help             help for init
      --project string   Project name
      --target string    Output instruction file (default "AGENTS.md")
      --yes              Accept defaults without prompting
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)

