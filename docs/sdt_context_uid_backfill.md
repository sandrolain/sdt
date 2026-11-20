## sdt context uid backfill

Stamp a missing uid on every eligible context/ document

### Synopsis

Stamp a missing UUIDv7 `uid` on every CLI-created context/ document
(every `context new` kind plus `context task`; wiki and architecture included).
Existing uids are never rewritten, so the command is idempotent. A real run
writes the marker `.sdt/cache/uid-backfill.json` so lint starts treating a
missing uid as a WARNING; --dry-run only reports.

Examples:
  sdt context uid backfill
  sdt context uid backfill --dry-run
  sdt context uid backfill --dry-run --format json

```
sdt context uid backfill [flags]
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

* [sdt context uid](sdt_context_uid.md)	 - Manage document identifiers (uid)
