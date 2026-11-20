## sdt context status get

Print the frontmatter status of a context/ document

### Synopsis

Print the frontmatter status field of an existing context/ document,
resolved by path (absolute or relative, with or without the .md suffix) or by
identity (--type/--slug, plus --number for decision or --phase/--plan for task
files).

Types without a status field (worklog, notes, tmp) are rejected.

Examples:
  sdt context status get context/plan/20260920-131900-plan-x.md
  sdt context status get plan/20260920-131900-plan-x
  sdt context status get --type decision --number 0001 --slug auth-choice
  sdt context status get --type analysis --slug backend --format json

```
sdt context status get <ref> [flags]
```

### Options

```
  -h, --help[=true|false]   help for get
      --number string       Decision number (type decision)
      --phase string        Phase for a task file (type tasks)
      --plan string         Plan reference for a task file (type tasks)
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

* [sdt context status](sdt_context_status.md)	 - Summarize context/ documents per type with next step
