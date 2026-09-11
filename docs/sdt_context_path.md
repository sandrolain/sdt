## sdt context path

Print the path for a context/ work file

### Synopsis

Print the full path of a context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis/worklog/notes/archive (<YYYYMMDD-HHMMSS>-<slug>.md),
tasks (<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md with --phase <n> and
--plan), tmp (<slug>), architecture (<slug>.md),
adr (<NNNN>-<slug>.md with --number).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type tasks --phase 1 --plan 20260911-155545-plan-context-file-formats-cli.md
  sdt context path --type plan --format json
  sdt context path --type adr --number 0001 --slug auth-choice

```
sdt context path [flags]
```

### Options

```
  -h, --help            help for path
      --number string   Number for type adr (4-digit NNNN)
      --phase string    Phase for type tasks (plan phase number, e.g. 1 or 1a)
      --plan string     Plan reference for type tasks (plan file or standalone slug)
      --slug string     Slug (sanitized)
      --type string     Type: plan|analysis|worklog|notes|tasks|tmp|archive|architecture|adr
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)

