## sdt learn eval

Run the behaviour cases under context/learning/evals (read-only)

### Synopsis

Run the deterministic grading and scheduling cases and report each verdict.

WHEN — after changing the grader, the schedule policy or a lesson's key, to prove
the contract still holds.
WHEN NOT — you want to grade one real answer: use sdt learn answer.

RETURNS — one row per case (ok or the expected and actual verdict), and a count.
A file holds a YAML list of cases; with no argument every `*.yaml` under
`context/learning/evals/` runs. Exit 1 when a case fails. Reads only: no
learner state is written.

EXAMPLE — sdt learn eval
          sdt learn eval context/learning/evals/grading.yaml --format json

```
sdt learn eval [file...] [flags]
```

### Options

```
  -h, --help[=true|false]   help for eval
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
