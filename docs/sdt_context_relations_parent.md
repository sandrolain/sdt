## sdt context relations parent

Print the recorded parent of a document

### Synopsis

Print the parent a document records in its frontmatter — `plan_id` for a
task file, `analysis_id` for a plan — resolved by path or identity, with the
parent's kind, title and status. Read-only; a document with no parent says so.

Examples:
  sdt context relations parent context/tasks/<task-file>.md
  sdt context relations parent plan/<plan-file> --format json

```
sdt context relations parent <doc-ref> [flags]
```

### Options

```
  -h, --help[=true|false]   help for parent
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
