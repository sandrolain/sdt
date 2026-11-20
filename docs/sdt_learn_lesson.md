## sdt learn lesson

Compose a lesson from its page and its lesson document (read-only)

### Synopsis

Compose the teaching bundle for a concept: objectives, pacing, prerequisites,
the worked example with its span, and the assessment items.

WHEN — a tutor or a portal needs the lesson as data, or the content must be
checked before it is used.
WHEN NOT — you want to know whether the concept can be taught at all: `sdt learn check`.
WHEN NOT — you want the page itself: open the wiki page.

RETURNS — the bundle (text, or --format json for a caller). `--validate` walks
every lesson and reports what would make it unusable: a missing page, a claim the
page does not carry, an objective index out of range, an objective with no item,
an empty expected or rubric, a duplicate item id. `--coverage` prints the
objective to item matrix. Exit 1 on any finding. Reads only.

EXAMPLE — sdt learn lesson architecture/four-key-metrics --format json
          sdt learn lesson --validate
          sdt learn lesson --coverage

```
sdt learn lesson [concept] [flags]
```

### Options

```
      --coverage[=true|false]   Print the objective to item matrix (read-only)
  -h, --help[=true|false]       help for lesson
      --validate[=true|false]   Validate every lesson against its page (read-only)
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
