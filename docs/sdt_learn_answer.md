## sdt learn answer

Grade one answer against an item's frozen key (read-only)

### Synopsis

Grade one answer deterministically and say which rule decided it.

WHEN — an answer has been given and must be checked before anything is recorded.
WHEN NOT — you want the questions: use sdt learn quiz. You want the key in bulk:
sdt learn quiz --with-key.

RETURNS — the grade (`correct`, `partially_correct`, `incorrect` or
`ungradable`), the rule that produced it (`exact`, `numeric`,
`keywords`, `manual` or the `auto` rule the item resolved to), the frozen key
and the rubric. Exit 0 for a correct or partially correct answer, 1 for an
incorrect one, 4 when the answer cannot be graded, the concept or item is not in
the corpus, or the item's bank is not current (a `key_hash` or `hash` that no longer
matches its row). Reads only; `sdt learn record` is what writes the verdict.

EXAMPLE — sdt learn answer --concept architecture/four-key-metrics --item fkm-3 --answer "5.56%"
          sdt learn answer --concept architecture/four-key-metrics --item fkm-1 --answer "deployment frequency" --format json

```
sdt learn answer --concept <concept> --item <item> --answer <text> [flags]
```

### Options

```
      --answer string       The answer to grade
      --concept string      Wiki page id (or title) of the concept
  -h, --help[=true|false]   help for answer
      --item ## Items       Item id from the concept's assessment (## Items)
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
