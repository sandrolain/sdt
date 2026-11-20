## sdt context relations similar

List the documents semantically nearest to a document (deprecated: use `relations similar`)

### Synopsis

List the documents whose content is nearest to <doc> in the shipped
semantic index, using the persisted vector snapshot (offline, no model load).
The result is advisory and degrades to a message when no snapshot exists.

Examples:
  sdt context related context/wiki/sdt-context-memory.md
  sdt context related notes/2026-01-01-x.md --limit 5 --format json

```
sdt context relations similar <doc> [flags]
```

### Options

```
  -h, --help[=true|false]   help for similar
      --limit int           Maximum number of neighbours to list (default 10)
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
