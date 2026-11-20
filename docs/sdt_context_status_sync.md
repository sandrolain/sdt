## sdt context status sync

Recompute derived document statuses across the store

### Synopsis

Reconcile declared and derived state across context/: a task file from its
checklist, a plan from its sourcing task files (plus its own checklists), an
analysis from its plans. It is idempotent — a clean store reports "nothing to
do" — and never touches archived/draft/abandoned documents or a document with
no children. --dry-run reports the flips without writing; --doc scopes the pass
to one subtree (the document and its ancestors).

Examples:
  sdt context sync
  sdt context sync --dry-run
  sdt context sync --doc plan/<plan-file>

```
sdt context status sync [flags]
```

### Options

```
      --doc string             Reconcile only this document and its ancestors
      --dry-run[=true|false]   Report without writing anything
  -h, --help[=true|false]      help for sync
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

* [sdt context status](sdt_context_status.md)	 - Summarize context/ documents per type with next step
