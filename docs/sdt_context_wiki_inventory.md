## sdt context wiki inventory

Validate concept-inventory notes (counters, rows, claims, scope)

### Synopsis

Validate the concept-inventory notes of a book-scale ingestion.

A note declares its counters and a "## Paged concepts" table:

  inventory:
    units: 223
    candidates: 39
    paged: 37
    merged: 2
    inline: 62
    declined: 0
    pending: 0

  | id | concept | disposition | target | claim | source |

The command checks the arithmetic (candidates = paged + merged + inline +
declined + pending), resolves every page/merge row to a live concept page and to
an existing {#claim-N} anchor on it, and requires every live concept page citing
the note's source to have a row. A pending count is a finding: the run is
incomplete.

With no argument every note whose note_type is concept-inventory is checked.
Exit 0 clean, 1 findings.

--write recomputes page/merge/declined/pending from the top table; inline is
preserved when the top table has no inline rows, because the chapter tables are
not a complete candidate list. --sync-index also repairs the index page.

Examples:
  sdt context wiki inventory
  sdt context wiki inventory notes/20261023-151134-ea-concept-inventory.md
  sdt context wiki inventory --write --sync-index --page sources/book --dry-run

```
sdt context wiki inventory [note...] [flags]
```

### Options

```
      --dry-run[=true|false]      With --write, print the diff without writing
  -h, --help[=true|false]         help for inventory
      --page string               Index page for --sync-index (default: the unique entity page citing the source)
      --sync-index[=true|false]   With --write, also update the index page's concepts block
      --write[=true|false]        Recompute the note counters from its ## Paged concepts table
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
