## sdt agent init

Bootstrap an SDT-managed project for AI agents

### Synopsis

Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                           project identity (project/group)
  AGENTS.md                           instructions block + optional write-once project template
  context/plan|worklog|notes|tasks|archive|tmp|scripts  working directories
  context/architecture/      living architecture documentation (no date)
  context/decisions/         numbered ADRs (NNNN-<slug>.md, append-only)
  context/questions/         open questions awaiting a user decision
  context/analysis/          analysis documents and implementation plans
  context/instructions/      per-type instruction/template files
  .gitignore                          ignores chosen context dirs (current dir)

The command is idempotent and non-destructive: a second run fills in missing
content and never overwrites or removes existing files. Use --force to refresh
generated content and remove obsolete instruction files.

The .gitignore entries for the context working directories are decided
interactively: you are asked whether to ignore them at all, and which entries
(tmp/, docs/, or the whole context/ directory). The file is created or
updated in the execution directory — the same directory as .sdt.yaml — even
when it is not a git repository; parent directories are never resolved. Use
--gitignore none|tmp|docs|work|context to pick non-interactively; work (tmp/ +
docs/ entries) is the default. --yes accepts that default without prompting.

The write-once project block (<!-- sdt:begin:project -->) is optional: you
are asked whether to insert it (default no), or pass --project-block to insert
it non-interactively. Declining never removes an existing block; re-running
never modifies a present block. The agent fills empty sections from project
evidence, asking first (see the hard rules in the instructions block).

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
      --gitignore string   Which context entries to add to .gitignore (none, tmp, docs, work, context); prompts interactively when omitted
      --group string       Group name
  -h, --help               help for init
      --project string     Project name
      --project-block      Insert the write-once <!-- sdt:begin:project --> block (asks interactively when omitted)
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

