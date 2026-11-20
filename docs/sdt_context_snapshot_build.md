## sdt context snapshot build

Build or refresh the semantic vector snapshot

### Synopsis

Build the semantic vector snapshot over the current context/ search index,
reusing unchanged vectors from the existing snapshot. No query is needed; the
embedding model is loaded from the local cache (default BASE8M, override with
--semantic-model).

Examples:
  sdt context snapshot build
  sdt context snapshot build --semantic-model BASE2M --format json

Under --progress human or json the embed loop narrates itself on stderr: one
throttled tick per batch, so a cold build no longer looks hung.

```
sdt context snapshot build [flags]
```

### Options

```
  -h, --help[=true|false]       help for build
      --semantic-model string   Embedding model (default BASE8M)
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

* [sdt context snapshot](sdt_context_snapshot.md)	 - Manage the derived semantic vector snapshot
