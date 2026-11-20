## sdt context relations backfill

Derive the typed parent relations from sources

### Synopsis

Walk the `sources` derivation chain of every plan and task file and stamp
the typed parent relation in both directions: `analysis_id`/`plans_ids` and
`plan_id`/`tasks_ids`. Requires the uid backfill first (the edges are keyed
by uid). Idempotent; --dry-run only reports.

Examples:
  sdt context relations backfill
  sdt context relations backfill --dry-run --format json

```
sdt context relations backfill [flags]
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

* [sdt context relations](sdt_context_relations.md)	 - Manage typed parent relations
