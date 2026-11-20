## sdt context template

Print the per-type instruction file for a context type

### Synopsis

Print the content of context/instructions/<type>.md for one document
type from the registered set (plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decision|tasks|wiki|lesson|assessment|theme|syllabus|practice|briefing). Read-only:
the CLI never writes documents.

Examples:
  sdt context template --type decision
  sdt context template --type plan --format json

```
sdt context template [flags]
```

### Options

```
  -h, --help[=true|false]   help for template
      --type string         Type: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decision|tasks|wiki|lesson|assessment|theme|syllabus|practice|briefing
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
