## sdt context checklist

Manage checklist item identifiers

### Synopsis

Manage the stable identifiers of context/ checklist items.

  sdt context checklist backfill [--dry-run]   stamp a missing id anchor on every
                                               checklist item (idempotent; writes
                                               .sdt/cache/checklist-backfill.json)

### Options

```
  -h, --help[=true|false]   help for checklist
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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
* [sdt context checklist backfill](sdt_context_checklist_backfill.md)	 - Stamp a missing id anchor on every context/ checklist item
