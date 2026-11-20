## sdt context task release

Release a task file claim

### Synopsis

Remove the claimed_by/claimed_at claim from a plan's task file and set it
back to pending, so it returns to the frontier.

```
sdt context task release [flags]
```

### Options

```
  -h, --help[=true|false]   help for release
      --phase string        Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)
      --plan string         Plan reference (plan file; default: latest active plan; custom slug for standalone)
      --stream string       Split-file label (kebab-case); selects <slug-plan>-<stream>.md
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

* [sdt context task](sdt_context_task.md)	 - Manage plan task checklists
