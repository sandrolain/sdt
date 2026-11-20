## sdt context memo due

List the rules and memos due today

### Synopsis

List the rules whose cadence has elapsed and the memos whose due date has
arrived, as of today (or --today). Text output is empty when nothing is due;
the exit status is 0 unless --check is given, which returns non-zero when
something is due (advisory otherwise).

```
sdt context memo due [flags]
```

### Options

```
      --check[=true|false]   Exit non-zero when something is due (for scripts)
  -h, --help[=true|false]    help for due
      --today string         Reference day (YYYY-MM-DD; default: today, UTC)
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
