## sdt context rename

Rename a context/ document (slug change)

### Synopsis

Rename an existing context/ document: recompute its path by the kind
scheme (dated kinds keep their creation-time prefix, bare/architecture and tmp
keep their dir, wiki accepts subpath slugs) and rewrite every reference to it
across context/ markdown and index.md.

Decisions are append-only (immutable filename) and task files are managed by
`sdt context task` — both are rejected.

Examples:
  sdt context rename context/analysis/20260920-130000-x.md --slug backend
  sdt context rename context/wiki/auth.md --slug backend/session
  sdt context rename context/plan/20260920-131900-plan-x.md --slug plan-y --dry-run

```
sdt context rename <ref> --slug <new> [flags]
```

### Options

```
      --dry-run[=true|false]   Preview without changing anything
  -h, --help[=true|false]      help for rename
      --slug string            New slug (target name)
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
