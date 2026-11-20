## sdt context memo add memo

Add a one-off memo

```
sdt context memo add memo [flags]
```

### Options

```
      --action string       Optional project command or hint
      --due string          Due day (YYYY-MM-DD)
  -h, --help[=true|false]   help for memo
      --id string           Memo id (kebab-case slug, unique across rules and memos)
      --source string       Optional document reference
      --summary string      One-line description of the reminder
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

* [sdt context memo add](sdt_context_memo_add.md)	 - Add a rule or a memo
