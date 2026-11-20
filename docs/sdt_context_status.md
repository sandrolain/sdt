## sdt context status

Summarize context/ documents per type with next step

### Synopsis

Summarize the context/ knowledge: per-type document count and the
recommended next step (read / write / verify). Useful at session start after
reindex. Ends with a store-completeness verdict (INCOMPLETE vs CLEAN).

Examples:
  sdt context status
  sdt context status --format json

```
sdt context status [flags]
```

### Options

```
  -h, --help[=true|false]   help for status
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
* [sdt context status get](sdt_context_status_get.md)	 - Print the frontmatter status of a context/ document
* [sdt context status set](sdt_context_status_set.md)	 - Set the frontmatter status of a context/ document
* [sdt context status sync](sdt_context_status_sync.md)	 - Recompute derived document statuses across the store
* [sdt context status vocab](sdt_context_status_vocab.md)	 - Print the per-kind status vocabulary
