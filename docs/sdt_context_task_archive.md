## sdt context task archive

Archive the active task list to context/archive/

```
sdt context task archive [flags]
```

### Options

```
  -h, --help           help for archive
      --phase string   Phase number from the plan, e.g. 1 or 1a (required)
      --plan string    Plan reference (plan file; default: latest active plan; custom slug for standalone)
      --slug string    Archive slug (default: from objective)
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

* [sdt context task](sdt_context_task.md)	 - Manage per-phase task checklists

