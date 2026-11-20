## sdt context wiki audit

Print the receipt block, page/claim totals and inventory status

### Synopsis

Generate the auditable receipt for one book-scale source.

The command is read-only: it builds the same coverage report as
"sdt context wiki coverage", then prints the receipt block, the number of pages
that cite the artifact, their claim anchors, and (with --inventory) the
inventory check. It never writes the page.

Examples:
  sdt context wiki audit sources/the-hard-parts --artifact refs/converted/x.md
  sdt context wiki audit sources/the-hard-parts --artifact refs/converted/x.md --inventory notes/x.md --format json

```
sdt context wiki audit <index-page> --artifact <artifact> [flags]
```

### Options

```
      --artifact string     Converted artifact the page represents (required)
      --from stringArray    Page carrying the citations (repeatable; default: every wiki page citing the artifact)
  -h, --help[=true|false]   help for audit
      --inventory string    Concept-inventory note to check (optional)
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
