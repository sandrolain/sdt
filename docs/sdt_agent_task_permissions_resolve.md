## sdt agent task permissions resolve



```
sdt agent task permissions resolve <task-id> <request-id> [flags]
```

### Options

```
  -h, --help[=true|false]   help for resolve
      --mandate string      Existing mandate reference required for a principal decision
      --option string       Exact offered permission option id
      --rationale string    Principal decision rationale
      --source string       Decision source: principal or user (explicit user response) (default "principal")
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

* [sdt agent task permissions](sdt_agent_task_permissions.md)	 - Inspect or resolve ACP permission requests
