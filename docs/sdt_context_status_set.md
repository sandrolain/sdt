## sdt context status set

Set the frontmatter status of a context/ document

### Synopsis

Set the frontmatter status field of an existing context/ document,
resolved by path or identity like `status get`. The value is validated
against the kind's status vocabulary (from the instruction contracts). When
the kind requires `updated` it is refreshed to now.

Types without a status field (worklog, notes, tmp) are rejected. Decisions are
append-only: only the status field changes, never the body or filename.

Examples:
  sdt context status set context/analysis/20260920-130000-x.md --status archived
  sdt context status set --type decision --number 0001 --slug auth-choice --status accepted
  sdt context status set --type proposal --slug provenance --status review

```
sdt context status set <ref> --status <v> [flags]
```

### Options

```
      --agent string              Provenance: agent/tool that performed the mutation
      --force[=true|false]        Set a terminal status even when derived state disagrees
  -h, --help[=true|false]         help for set
      --model string              Provenance: model id that performed the mutation
      --no-cascade[=true|false]   Do not propagate completion to parent documents
      --number string             Decision number (type decision)
      --phase string              Phase for a task file (type tasks)
      --plan string               Plan reference for a task file (type tasks)
      --reason string             With a terminal status: record why the derived state disagrees (the close path)
      --session string            Provenance: session id for traceability
      --slug string               Slug for identity resolution
      --status string             New status value (validated against the kind vocabulary)
      --stream string             Split-file label for a task file (type tasks)
      --type string               Document type for identity resolution
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
