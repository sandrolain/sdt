## sdt learn check

Report whether a concept can be taught, and why not (read-only)

### Synopsis

Answer one question per concept: can a lesson be built from this page?

WHEN — before teaching, before composing a lesson, or when a portal decides what
to offer.
WHEN NOT — you want the concepts behind a topic (learn inspect) or the lesson
itself (learn lesson).

RETURNS — one verdict per concept, teachable or refused, with its reasons and the
page's numbers. The six rules are about what the content can support: the page is
`active`, it carries at least four claims, its maturity is not `wip`, it has
a `learning:` block, a lesson document exists, and every objective has an item.
Provenance is a note, never a rule: a page without `verified_at` is still
teachable, reported as `unverified`, and `--verified-only` merely narrows the
report to stamped concepts. Exit 1 when a reported concept is refused.

EXAMPLE — sdt learn check architecture/four-key-metrics
          sdt learn check architecture/four-key-metrics --format json
          sdt learn check --verified-only

```
sdt learn check <concept>... [flags]
```

### Options

```
  -h, --help[=true|false]            help for check
      --verified-only[=true|false]   Report only concepts that carry a verified_at stamp (a filter, not a rule)
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
