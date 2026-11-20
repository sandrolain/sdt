## sdt agent init

Bootstrap an SDT-managed project for AI agents

### Synopsis

Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                           project identity (project/group)
  AGENTS.md                           instructions block + optional write-once project template
  context/plan|worklog|notes|tasks|trash|tmp|scripts  working directories
  context/proposals/     proposals awaiting review
  context/prompts/       tracked prompts (run/review)
  context/research/      research notes and findings
  context/architecture/  living architecture documentation (no date)
  context/decisions/     numbered decisions (NNNN-<slug>.md, append-only)
  context/questions/     open questions awaiting a user decision
  context/analysis/      analysis documents and implementation plans
  context/wiki/          wiki pages (grouped in subpaths, no date)
  context/instructions/  per-type instruction/template files
  context/roles/         generated role profiles (sdt agent roles init)
  .gitignore                          ignores chosen context dirs (current dir)

The command is idempotent and non-destructive: a second run fills in missing
content and never overwrites or removes existing files. Use --force to refresh
the generated (marker-delimited) content while preserving edits outside the
markers, and to move obsolete instruction files to context/trash/
instead of deleting them.

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
      --force[=true|false]           Refresh generated template content
      --gitignore string             Which context entries to add to .gitignore (none, tmp, docs, work, context); prompts interactively when omitted
      --group string                 Group name
  -h, --help[=true|false]            help for init
      --project string               Project name
      --project-block[=true|false]   Insert the write-once <!-- sdt:begin:project --> block (asks interactively when omitted)
      --target string                Output instruction file (default "AGENTS.md")
      --with stringArray             Optional generation profile to add (repeatable): knowledge, ui, vector, slides, mindmap, diagram, browser, security, roles
      --yes[=true|false]             Accept defaults without prompting
```

### Options inherited from parent commands

```
      --file string             Input File
      --format string           Output format: text|json|yaml (default "text")
      --inb64 bytesBase64       Input Base 64
      --input string            Input String
      --log-format string       Log format: text|json (default "text")
      --no-color[=true|false]   Disable ANSI color codes
      --no-env[=true|false]     Do not load a project-root .env file
      --progress string         Progress narration on stderr: auto|human|json|none (default "auto")
      --quiet[=true|false]      Suppress informational messages, only output result
```

### SEE ALSO

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)
