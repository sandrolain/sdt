## sdt doc2md set

Enrich a kept document's summary and links

### Synopsis

Set the summary and links of a kept document and refresh both index
projections. <file> is the basename of a file under context/refs/converted/
(or a path inside it). It is the only mutator of the kept corpus; a document
with no relation simply carries no links.

Examples:
  sdt doc2md set report-ab12cd34ef56.md --summary "Q3 financials"
  sdt doc2md set report-ab12cd34ef56.md --link analysis/<analysis-file>.md

```
sdt doc2md set <file> [flags]
```

### Options

```
      --force[=true|false]   Allow a byte-changing frontmatter rewrite of a cited artifact (the citing documents are named either way)
  -h, --help[=true|false]    help for set
      --link stringArray     Related document path (repeatable)
      --summary string       Document summary
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
      --timeout duration        Per-process timeout (default 2m0s)
```

### SEE ALSO

* [sdt doc2md](sdt_doc2md.md)	 - Convert documents to Markdown with the first runnable converter
