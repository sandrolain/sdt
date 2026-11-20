## sdt context task progress

Show a task file's per-phase progress (N/M, current phase, blocked ids)

### Synopsis

Show the per-phase progress of the active plan's task file: each phase's
done/total count, the blocked item ids, and which phase is current (the first
unfinished one). Read-only.

Examples:
  sdt context task progress
  sdt context task progress --plan plan/<plan-file>

```
sdt context task progress [flags]
```

### Options

```
  -h, --help[=true|false]   help for progress
      --plan string         Plan reference (plan file; default: latest active plan)
      --stream string       Split-file label (kebab-case)
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

* [sdt context task](sdt_context_task.md)	 - Manage plan task checklists
