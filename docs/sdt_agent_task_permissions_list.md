## sdt agent task permissions list



```
sdt agent task permissions list <task-id> [flags]
```

### Options

```
      --escalated[=true|false]   Show only requests carrying an escalation reason
  -h, --help[=true|false]        help for list
      --outcome string           Filter recorded disposition, such as pending, resolved or stale
      --source string            Filter decision source, including principal, user or policy
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
