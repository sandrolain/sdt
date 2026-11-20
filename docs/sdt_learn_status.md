## sdt learn status

Report goals, coverage, weak areas and due reviews (read-only)

### Synopsis

Report how far the learner has got against the wiki.

WHEN — you need the whole learning picture: goals, how many concepts are recorded,
what is weak and what is due.
WHEN NOT — you want to resolve a topic: use sdt learn inspect.

RETURNS — learner, goal count, recorded-concept count against the wiki page count,
coverage and mastery as two separate lines, plus the weak and due concepts. Coverage
is exposure (`assessed`, `covered`, `unseen`); mastery is the standing of
the recorded concepts (`solid`, `partial`, `weak`, `unknown`), and the two
are never folded into one number. An absent learner state reports zeros rather than
failing. Reads only.

EXAMPLE — sdt learn status
          sdt learn status --format yaml

```
sdt learn status [flags]
```

### Options

```
  -h, --help[=true|false]   help for status
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
