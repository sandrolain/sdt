## sdt context wiki lint

Validate the wiki knowledge graph and source markers

### Synopsis

Validate the knowledge pipeline: wiki/ pages (schema, closed relation
verbs, link resolution, unique titles, depends_on cycles, supersede semantics,
claim citations, tags, concept budget, optional markdown-ld JSON), plus the
immutable source markers under ingestion/ (status: pending) and refs/
(status: archived).

wiki/ is scanned recursively (wiki/<context>/<slug>.md allowed): a page's id
must equal its relative subpath minus ".md" (wiki/backend/auth.md → id:
backend/auth).

Exit codes: 0 clean, 1 warnings only, 2 errors.

Examples:
  sdt context wiki lint
  sdt context wiki lint --format json

```
sdt context wiki lint [flags]
```

### Options

```
  -h, --help   help for lint
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)

