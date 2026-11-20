## sdt context deprecate

Retire a document by changing its status, without moving it

### Synopsis

Retire a document: set the retired status its own type's vocabulary carries,
and nothing else. The file does not move, so every reference to it keeps
resolving and a reader answers "is this still valid" from the state rather than
from where the document sits.

WHEN — a document is still worth reading and no longer current: an analysis a
later analysis replaced, a decision no longer in force, a prompt nobody should
reach for again. Pass `--by <ref>` to name the successor when one document
replaced another; the value is recorded in `superseded_by` and must resolve,
because a dangling successor is worse than none.

WHEN NOT — the file itself must go: `deprecate` keeps it, by design. A type
whose vocabulary carries no retired value (`questions`, `commands`) is
refused by name rather than given another type's word, and a type with no status
field at all (worklog, notes) has no retired state to set.

RETURNS — the path, the type, the status it had and the one it now has, plus the
successor when one was named. The retired value per type is declared in the type
registry, so the answer here and the value `sdt context vocab` prints cannot
drift apart.

EXAMPLE — `sdt context deprecate analysis/20260101-000000-old --by analysis/20260202-000000-new`
          `sdt context deprecate --type prompt --slug legacy --dry-run`

```
sdt context deprecate <ref> [flags]
```

### Examples

```
  sdt context deprecate analysis/20260101-000000-old
  sdt context deprecate analysis/20260101-000000-old --by analysis/20260202-000000-new
  sdt context deprecate --type decision --number 0007 --slug auth-choice
```

### Options

```
      --agent string           Provenance: agent/tool that performed the mutation
      --by string              Reference of the document that supersedes this one
      --dry-run[=true|false]   Report the transition without writing it
  -h, --help[=true|false]      help for deprecate
      --model string           Provenance: model id that performed the mutation
      --number string          Decision number (type decision)
      --phase string           Phase for a task file (type tasks)
      --plan string            Plan reference for a task file (type tasks)
      --session string         Provenance: session id for traceability
      --slug string            Slug for identity resolution
      --stream string          Split-file label for a task file (type tasks)
      --type string            Document type for identity resolution
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
