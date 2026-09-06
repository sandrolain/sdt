## sdt context template

Print the per-type instruction file for a context type

### Synopsis

Print the content of sdt.context/instructions/<tipo>.md for one document
type (analysis, plan, tasks, adr, architecture, worklog, notes). Read-only: the
CLI never writes documents.

Examples:
  sdt context template --type adr
  sdt context template --type plan --format json

```
sdt context template [flags]
```

### Options

```
  -h, --help          help for template
      --type string   Type: analysis|plan|tasks|adr|architecture|worklog|notes
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

