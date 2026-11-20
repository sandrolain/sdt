## sdt context memo add rule

Add a recurring rule

```
sdt context memo add rule [flags]
```

### Options

```
      --action string           Optional project command or hint for the operation
      --disabled[=true|false]   Add the rule disabled (excluded from the due check)
      --every-days int          Cadence in days (positive integer)
  -h, --help[=true|false]       help for rule
      --id string               Rule id (kebab-case slug, unique across rules and memos)
      --last-done string        Day it was last done (YYYY-MM-DD; omit for due immediately)
      --source string           Optional document reference for the operation
      --summary string          One-line description of the operation
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
