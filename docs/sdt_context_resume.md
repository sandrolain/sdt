## sdt context resume

Show the recorded execution state to resume from (read-only)

### Synopsis

Show what the context/ documents say about the work in flight: the open
plan(s), each task file's phases with item counts, the blocked items with the
reason recorded on them, and what the reconciler found stale or orphaned.

recorded state only: a session that wrote nothing looks the same as one that never started. Nothing here is a live process view: it reports the
paperwork, so use it to decide what to open next, never to claim what is running.

Examples:
  sdt context resume
  sdt context resume --plan 20261005-example.md
  sdt context resume --stale-days 30
  sdt context resume --format json

```
sdt context resume [flags]
```

### Options

```
  -h, --help[=true|false]        help for resume
      --plan string              Plan reference to report (default: every active plan)
      --stale-days in-progress   Days before an in-progress task file is reported as stale (default 14)
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
