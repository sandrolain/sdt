## sdt agent init

Bootstrap an SDT-managed project for AI agents

### Synopsis

Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                           project identity (project/group)
  AGENTS.md                           single tagged block of general instructions
  sdt.context/plan|worklog|notes|tasks|archive|tmp  working directories
  sdt.context/analysis/          analysis documents and implementation plans
  sdt.context/memory/            persistent file-based memory (README.md + index.md + pages/)
  sdt.context/instructions/      CLI usage files (project, memory, reference, cli usage)
  .gitignore                          ignores chosen sdt.context dirs (git repos only)

The command is idempotent and non-destructive: a second run fills in missing
content and never overwrites or removes existing files. Use --force to refresh
generated content and remove obsolete instruction files.

When the current directory is inside a git repository, the .gitignore entries
for the sdt.context working directories are decided interactively: you are asked
whether to ignore them at all, and which entries (tmp/, docs/, or the whole
sdt.context/ directory). Use --gitignore none|tmp|docs|work|context to pick
non-interactively; work (tmp/ + docs/ entries) is the default. --yes accepts
that default without prompting.

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
      --force              Refresh generated template content
      --gitignore string   Which sdt.context entries to add to .gitignore (none, tmp, docs, work, context); prompts interactively when omitted
      --group string       Group name
  -h, --help               help for init
      --project string     Project name
      --target string      Output instruction file (default "AGENTS.md")
      --yes                Accept defaults without prompting
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

