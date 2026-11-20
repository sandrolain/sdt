## sdt context task claim

Claim a task file for this session

### Synopsis

Record a durable claim (claimed_by/claimed_at) on a plan's task file and
mark it in progress, so a concurrent session reads the frontier from
`sdt context resume` and skips it. Release it with `task release`.

Examples:
  sdt context task claim --plan <plan-file> --agent <agent-name>
  sdt context task release --plan <plan-file>

```
sdt context task claim [flags]
```

### Options

```
      --agent string        Claiming agent/session id (default: $SDT_AGENT, $USER)
  -h, --help[=true|false]   help for claim
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
