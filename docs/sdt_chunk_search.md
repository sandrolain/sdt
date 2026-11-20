## sdt chunk search

Rank an artifact's sections against a query, transiently

### Synopsis

Rank the sections of a converted artifact against a query.

The artifact is split in memory with the same fence-aware splitter the corpus
uses (mdstruct, through internal/distill), embedded into a throwaway in-memory
collection, and discarded at exit. Nothing is written under .sdt/cache: the
corpus index and the vector snapshot are read-only inputs.

Each result carries the section anchor and a refs/<file>@<sha8>:<start>-<end>
locator, so it can be cited without re-reading the book. --with-wiki also ranks
the wiki pages' sections, reusing the persisted snapshot vectors read-only, and
fuses the two lists with RRF.

When no embedding model is cached (sdt models fetch), the command degrades to
lexical ranking and says so; it still succeeds.

Examples:
  sdt chunk search refs/converted/book-abcdef123456.md "how is coupling measured?"
  sdt chunk search refs/converted/book.md "saga compensation" --with-wiki --top 5
  sdt chunk search refs/converted/book.md "granularity" --format json

```
sdt chunk search <artifact> <query> [flags]
```

### Options

```
      --all-units[=true|false]   Include front/back matter (excluded by default)
      --budget int               Snippet token budget (0 = whole section) (default 120)
  -h, --help[=true|false]        help for search
      --levels int               Deepest heading level treated as a unit (default 3)
      --model string             Embedding model: BASE2M|BASE8M|MULTILINGUAL128M (default BASE8M)
      --top int                  Number of results (default 8)
      --with-wiki[=true|false]   Also rank wiki sections from the persisted snapshot, read-only
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

* [sdt chunk](sdt_chunk.md)	 - Transient chunk search over an artifact (no index written)
