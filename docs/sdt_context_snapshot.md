## sdt context snapshot

Manage the derived semantic vector snapshot

### Synopsis

Manage the derived semantic vector snapshot (vectors.gob.gz under
.sdt/cache) that backs the offline neighbour surfaces (sdt context related, the
semantic map).

### Options

```
  -h, --help[=true|false]   help for snapshot
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
* [sdt context snapshot build](sdt_context_snapshot_build.md)	 - Build or refresh the semantic vector snapshot
