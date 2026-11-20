## sdt context wiki frontier

Report where the wiki is thin (read-only frontier score)

### Synopsis

Rank wiki pages by a frontier score: (out_degree − in_degree) ×
exp(−days_since_updated/30). A high score is a page the corpus points to rarely
and that has not moved — where knowledge is thin and deserves more. Read-only,
deterministic, model-free; reuses the shared wiki graph.

Examples:
  sdt context wiki frontier
  sdt context wiki frontier --limit 10 --format json

```
sdt context wiki frontier [flags]
```

### Options

```
  -h, --help[=true|false]   help for frontier
      --limit int           Maximum rows to report (0 = all) (default 20)
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

* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)
