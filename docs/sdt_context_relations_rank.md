## sdt context relations rank

Rank candidate documents of a type (deprecated: use `relations rank`)

### Synopsis

Rank the documents of a type by the signals the working-context resolution
ladder uses — a matching --status first, then most-recently updated — and print
them (path, status, updated).

This is a read-only helper for the workflow commands and the status/doctor hints;
it is deliberately NOT the resolution authority: a workflow command resolves its
subject via the prose ladder in context/commands/index.md and asks the user when
the result is not unique. resolve writes nothing.

Types: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decisions|tasks|wiki|lesson|assessment|theme|syllabus|practice|briefing.

Examples:
  sdt context resolve --type analysis --status active
  sdt context resolve --type plan --status active --format json

```
sdt context relations rank --type <t> [--status <s>] [flags]
```

### Options

```
  -h, --help[=true|false]   help for rank
      --status string       Prefer candidates carrying this status (validated against the type vocabulary)
      --type string         Document type to resolve candidates for
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
