## sdt context touch

Refresh the `updated` timestamp of a context/ document

### Synopsis

Refresh the `updated` frontmatter field of an existing context/
document, resolved by path or identity like `status get`. It is the
CLI-only way to record a body edit once `updated` is no longer
hand-stamped; no other field, the body or the filename is changed.

Types without an `updated` field (notes, decision) are rejected.

Examples:
  sdt context touch context/plan/<plan-file>.md
  sdt context touch plan/<plan-file>
  sdt context touch --type analysis --slug backend --format json

```
sdt context touch <ref> [flags]
```

### Options

```
      --agent string        Provenance: agent/tool that performed the mutation
  -h, --help[=true|false]   help for touch
      --model string        Provenance: model id that performed the mutation
      --number string       Decision number (type decision)
      --phase string        Phase for a task file (type tasks)
      --plan string         Plan reference for a task file (type tasks)
      --session string      Provenance: session id for traceability
      --slug string         Slug for identity resolution
      --stream string       Split-file label for a task file (type tasks)
      --type string         Document type for identity resolution
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
