## sdt learn review

List the due and weak concepts as a bounded review queue (read-only)

### Synopsis

Rank the recorded concepts by what needs revisiting.

WHEN — a review session starts, or you need the next concepts to revisit.
WHEN NOT — you want goals and coverage: use sdt learn status.

RETURNS — one row per queued concept: due first (review date reached), then weak
(concept not yet solid or carrying a misconception), each ordered by review date,
with the distance in days and the reason the date was chosen (`schedule_reason`).
--limit bounds the queue (0 = all). Deterministic; reads only.

EXAMPLE — sdt learn review
          sdt learn review --limit 5 --format json

```
sdt learn review [flags]
```

### Options

```
  -h, --help[=true|false]   help for review
      --limit int           Maximum rows to queue (0 = all)
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

* [sdt learn](sdt_learn.md)	 - Learn, review and record concepts against the local wiki
