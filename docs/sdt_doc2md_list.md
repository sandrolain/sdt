## sdt doc2md list

List the kept converted documents

### Synopsis

List the kept converted corpus. The rows are read from the per-file
frontmatter under context/refs/converted/ (never from index.md), so the
projection can never become a second source of truth. --format json|yaml
prints the machine view and adds each artifact's routing counts
(section_headings, lines) measured on the file, so older keeps can be
refilled from the listing alone.

Examples:
  sdt doc2md list
  sdt doc2md list --format json

```
sdt doc2md list [flags]
```

### Options

```
  -h, --help[=true|false]   help for list
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
