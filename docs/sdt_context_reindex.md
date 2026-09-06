## sdt context reindex

Regenerate sdt.context/index.md from frontmatter summaries

### Synopsis

Scan the sdt.context/ knowledge directories, read the mandatory frontmatter
summary of every document and regenerate sdt.context/index.md grouped by
relevance tier (essential, important, medium, operational, history).

Examples:
  sdt context reindex
  sdt context reindex --format json

```
sdt context reindex [flags]
```

### Options

```
  -h, --help   help for reindex
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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)

