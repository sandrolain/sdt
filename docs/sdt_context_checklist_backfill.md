## sdt context checklist backfill

Stamp a missing id anchor on every context/ checklist item

### Synopsis

Stamp a missing `<!-- c<N> -->` anchor on every checklist item of every
context/ document (task files, plan Phases/Completion criteria, questions and
any other checklist), normalize an item's anchor onto its checklist line, and
renumber a repeated `c<N>` so every id addresses exactly one item.
Existing distinct anchors are preserved, so the command is idempotent. A real run
writes the marker
`.sdt/cache/checklist-backfill.json` so lint starts treating a missing
anchor as a WARNING; --dry-run only reports.

Examples:
  sdt context checklist backfill
  sdt context checklist backfill --dry-run
  sdt context checklist backfill --dry-run --format json

```
sdt context checklist backfill [flags]
```

### Options

```
      --dry-run[=true|false]   Report without writing anything
  -h, --help[=true|false]      help for backfill
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

* [sdt context checklist](sdt_context_checklist.md)	 - Manage checklist item identifiers
