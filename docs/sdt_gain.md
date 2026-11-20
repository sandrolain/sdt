## sdt gain

Report what SDT actually did, counted from the local usage log

### Synopsis

Report the counted facts of this project's local usage log: how often each
command ran, how many of those runs failed, how many bytes came back and — for
the commands that report a count — how often an invocation found at least one
item.

WHEN — you want to know which verbs the work actually uses, which ones fail, and
whether the searches return anything, instead of guessing.

WHEN NOT — you want a cost estimate. No counterfactual saving is computed: the
log holds what happened, never what would have happened without SDT.

The log lives at <root>/.sdt/usage/usage-YYYY-MM.jsonl and holds the command
path, the exit code, the duration, the bytes written to stdout and an optional
hit count. It never holds an argument, a path, a query or any content.
`SDT_USAGE=off` disables it, as does `usage.enabled: false` in .sdt.yaml.

`gain` itself and the internal calls SDT makes on its own behalf are excluded
from the totals and counted separately; `--all` includes them.

```
sdt gain [flags]
```

### Examples

```
  sdt gain
  sdt gain --since 30d
  sdt gain --since all --json
```

### Options

```
      --all[=true|false]    Include this command and SDT's internal calls in the totals
  -h, --help[=true|false]   help for gain
      --json[=true|false]   Print the report as JSON
      --since string        Window: a duration (24h), days (7d) or all (default "7d")
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

* [sdt](sdt.md)	 - Smart Developer Tools
