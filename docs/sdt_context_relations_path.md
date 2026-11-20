## sdt context relations path

Show a document's lifecycle chain (deprecated: use `relations path`)

### Synopsis

Show a document's lifecycle chain, from the document up to its root, over the
recorded typed relations (task → plan → analysis). Read-only and deterministic:
the chain is assembled from what is recorded, never from memory. A document with
no parent is its own chain.

Examples:
  sdt context trace context/tasks/<task-file>.md
  sdt context trace plan/<plan-file> --format json

```
sdt context relations path <doc-ref> [flags]
```

### Options

```
  -h, --help[=true|false]   help for path
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
