## sdt context task add

Add a step to the active task list

```
sdt context task add <step> [flags]
```

### Options

```
      --agent string              Provenance: agent/tool that performed the mutation
  -h, --help[=true|false]         help for add
      --model string              Provenance: model id that performed the mutation
      --no-cascade[=true|false]   Do not propagate completion to parent documents
      --phase string              Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)
      --plan string               Plan reference (plan file; default: latest active plan; custom slug for standalone)
      --session string            Provenance: session id for traceability
      --stream string             Split-file label (kebab-case); selects <slug-plan>-<stream>.md
      --summary string            Summary for the checklist frontmatter (default: derived from the phase)
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
