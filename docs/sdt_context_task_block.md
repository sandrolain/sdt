## sdt context task block

Mark a task step blocked

```
sdt context task block <id> [flags]
```

### Options

```
  -h, --help            help for block
      --phase string    Phase for the checklist file (default plan) (default "plan")
      --reason string   Reason for blocking
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

* [sdt context task](sdt_context_task.md)	 - Manage per-phase task checklists (sdt.context/tasks/<phase>.md)

