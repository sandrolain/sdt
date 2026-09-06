## sdt context path

Print the path for a sdt.context/ work file

### Synopsis

Print the full path of a sdt.context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis/worklog/notes/archive (<YYYYMMDD-HHMMSS>-<slug>.md),
tasks (<phase>.md with --phase, default plan), tmp (<slug>),
architecture (<slug>.md).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type tasks --phase execution
  sdt context path --type plan --format json

```
sdt context path [flags]
```

### Options

```
  -h, --help           help for path
      --phase string   Phase for type tasks (checklist file name) (default "plan")
      --slug string    Slug (sanitized)
      --type string    Type: plan|analysis|worklog|notes|tasks|tmp|archive|architecture|decisions
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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)

