## sdt context wiki units

Enumerate a converted artifact's headings (units) and spans

### Synopsis

Enumerate a converted artifact's heading spans for a book-scale ingestion.

The unit is a heading outside a fenced code block; a container (chapter, part,
index, front/back matter) groups units and is reported but not counted. Levels
1-3 are read by default, matching the corpus convention that a chapter and its
sections demarcate the units; adjust with --levels.

The row carries the 1-based line span, so a page citation (:n or :n-m) maps onto
it directly. The stats block is what an ingestion receipt quotes: raw heading
lines (including code-fence comments), headings outside fences, section-level
headings (levels 2-3, the routing proxy) and body lines.

Examples:
  sdt context wiki units refs/converted/book-abcdef123456.md
  sdt context wiki units refs/converted/book-abcdef123456.md --format json
  sdt context wiki units refs/converted/book.md --levels 2 --containers '^Chapitre'
  sdt context wiki units refs/converted/book.md --match wiki/architecture --limit 3

With --match <dir> each unit is also ranked against the pages under that
directory with the shared corpus index, and the top --limit candidates follow
the unit line with the words they share. It is a proposal for the fold-in merge,
read-only: it never writes a page, reserves a slug or dispositions a unit.

```
sdt context wiki units <artifact> [flags]
```

### Options

```
      --containers string   Regular expression marking structural containers (default: the corpus convention)
  -h, --help[=true|false]   help for units
      --levels int          Deepest heading level counted as a unit (default 3)
      --limit int           Candidate pages per unit with --match (default 3)
      --match string        Rank each unit against the pages of a corpus directory (read-only)
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
