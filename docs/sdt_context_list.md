## sdt context list

List sdt.context/ work files

### Synopsis

List existing work files under sdt.context/ for a type, sorted by name
(chronological for timestamped files).

Types: plan, analysis, worklog, notes, tasks, archive.

Examples:
  sdt context list --type worklog
  sdt context list --type archive --format json

```
sdt context list [flags]
```

### Options

```
  -h, --help          help for list
      --type string   Type: plan|analysis|worklog|notes|tasks|archive
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

