## sdt context wiki coverage

Classify an artifact's units from the pages' citations (OK/PART/GAP)

### Synopsis

Audit a book-scale source's coverage from the destination's own citations.

Units are enumerated from the artifact (sdt context wiki units). Citations of the
form refs/<file>@<sha>:<lines> are read from the wiki pages that cite it (all
pages by default, or --from), and each unit is classified:

  OK    every substantive line of the span is cited (non-blank, outside fences)
  PART  some lines are cited
  GAP   none are cited

The disposition already written in the page's ## Structure list is kept; a unit
without one gets "gap" when GAP. --receipt prints the routing block and the
coverage line an ingestion receipt quotes. --write regenerates the frontmatter
coverage block and the ## Structure section; it is the only mutating mode and it
preserves every byte outside those two regions.

Examples:
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md --receipt
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md --write --dry-run

```
sdt context wiki coverage <index-page> --artifact <artifact> [flags]
```

### Options

```
      --artifact string        Converted artifact the page represents (required)
      --dispositions string    With --write, a YAML map of L<n>: no-op|not-conceptual overriding the Structure rows
      --dry-run[=true|false]   With --write, print the result without writing
      --from stringArray       Page carrying the citations (repeatable; default: every wiki page citing the artifact)
  -h, --help[=true|false]      help for coverage
      --inventory string       Concept-inventory note to derive the concepts block from (default: the page's recorded from)
      --receipt[=true|false]   Print the ingestion receipt's routing block
      --write[=true|false]     Regenerate the coverage block and ## Structure
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
