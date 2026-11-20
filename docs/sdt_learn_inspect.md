## sdt learn inspect

Resolve a topic to wiki concepts and their prerequisites (read-only)

### Synopsis

Resolve a topic against the wiki graph and report what covers it.

WHEN — you need the concepts behind a topic, what they depend on, and which the
learner has already recorded.
WHEN NOT — the corpus has no matching page: this reports what the corpus has and
stops; it never answers from recall.

RETURNS — one entry per matching page with its prerequisite ids and the learner's
recorded standing, ordered by page id. Reads only. --limit bounds the report
(0 = all).

EXAMPLE — sdt learn inspect "resolution order"
          sdt learn inspect backend/auth --format json

```
sdt learn inspect [topic] [flags]
```

### Options

```
  -h, --help[=true|false]   help for inspect
      --limit int           Maximum concepts to report (0 = all)
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

* [sdt learn](sdt_learn.md)	 - Learn, review and record concepts against the local wiki
