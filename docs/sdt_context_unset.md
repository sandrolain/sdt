## sdt context unset

Remove a top-level frontmatter key

### Synopsis

Remove a top-level frontmatter key from a context document, preserving every
other byte. Removing an absent key is a no-op. uid, analysis_id and plan_id are
refused without --force.

Examples:
  sdt context unset analysis/x.md categories

```
sdt context unset <doc> <key> [flags]
```

### Options

```
      --agent string         Provenance: agent/tool that performed the mutation
      --force[=true|false]   Remove a CLI-owned key (uid, analysis_id, plan_id)
  -h, --help[=true|false]    help for unset
      --model string         Provenance: model id that performed the mutation
      --number string        Decision number (type decision)
      --phase string         Phase for a task file (type tasks)
      --plan string          Plan reference for a task file (type tasks)
      --session string       Provenance: session id for traceability
      --slug string          Slug for identity resolution
      --stream string        Split-file label for a task file (type tasks)
      --type string          Document type for identity resolution
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
