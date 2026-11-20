## sdt context memo

Manage the planned-operation register (rules and memos)

### Synopsis

Manage context/memo.yaml: recurring rules (a cadence) and one-off memos
(a due date), together with the last-run state.

  sdt context memo add rule --id <id> --summary <s> --every-days <n>
  sdt context memo add memo --id <id> --summary <s> --due <YYYY-MM-DD>
  sdt context memo list
  sdt context memo due [--today <YYYY-MM-DD>] [--check]
  sdt context memo done <id>
  sdt context memo remove <id>

`due` prints nothing when nothing is due. It is advisory by default and
exits non-zero only with `--check`, for scripts.

### Options

```
  -h, --help[=true|false]   help for memo
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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
* [sdt context memo add](sdt_context_memo_add.md)	 - Add a rule or a memo
* [sdt context memo done](sdt_context_memo_done.md)	 - Record a rule as done today, or a memo as done
* [sdt context memo due](sdt_context_memo_due.md)	 - List the rules and memos due today
* [sdt context memo list](sdt_context_memo_list.md)	 - List the register entries
* [sdt context memo remove](sdt_context_memo_remove.md)	 - Remove a rule or a memo
