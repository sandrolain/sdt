## sdt context uid

Manage document identifiers (uid)

### Synopsis

Manage the immutable UUIDv7 `uid` identifier of context/ documents.

  sdt context uid backfill [--dry-run]   stamp a missing uid on every eligible document
                                         (idempotent; writes .sdt/cache/uid-backfill.json)

### Options

```
  -h, --help[=true|false]   help for uid
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
* [sdt context uid backfill](sdt_context_uid_backfill.md)	 - Stamp a missing uid on every eligible context/ document
