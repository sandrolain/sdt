## sdt agent task wait

Wait for delegated tasks through worker events

```
sdt agent task wait <task-id> [task-id...] [flags]
```

### Options

```
      --all[=true|false]    Wait for all supplied tasks to finish; return early for intervention
      --any[=true|false]    Wait for any supplied task to require attention
  -h, --help[=true|false]   help for wait
      --timeout duration    Maximum time to wait (default 1m0s)
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

* [sdt agent task](sdt_agent_task.md)	 - Delegate and inspect asynchronous ACP tasks
