## sdt agent task message send



```
sdt agent task message send <task-id> [flags]
```

### Options

```
      --artifact string     Name of a retained task artifact to reference
      --from string         Task participant sending the message
  -h, --help[=true|false]   help for send
      --text string         Coordination note; treated as untrusted context
      --to string           Task participant receiving the message
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

* [sdt agent task message](sdt_agent_task_message.md)	 - Exchange untrusted coordination notes within one task
