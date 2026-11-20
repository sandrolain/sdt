## sdt context reindex

Regenerate context/index.md and context/index.json from frontmatter summaries

### Synopsis

Scan the context/ knowledge directories, read the mandatory frontmatter
summary of every document and regenerate context/index.md grouped by
relevance tier (essential, important, medium, operational, history), plus
context/index.json — the same documents as a flat, queryable projection.

Query the JSON with sdt q, e.g.:
  sdt q get context/index.json `documents.#(objective=="structured-data-access")#.path`

Examples:
  sdt context reindex
  sdt context reindex --format json

```
sdt context reindex [flags]
```

### Options

```
  -h, --help[=true|false]   help for reindex
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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
