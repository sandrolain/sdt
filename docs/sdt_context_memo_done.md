## sdt context memo done

Record a rule as done today, or a memo as done

### Synopsis

Record that a rule was performed (sets last_done to today, or --today), or
that a memo is done. Call it only after the operation succeeded: it records a
claim, and SDT cannot verify the operation.

```
sdt context memo done <id> [flags]
```

### Options

```
  -h, --help[=true|false]   help for done
      --today string        Reference day for a rule (YYYY-MM-DD; default: today, UTC)
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

* [sdt context memo](sdt_context_memo.md)	 - Manage the planned-operation register (rules and memos)
