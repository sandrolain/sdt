## sdt context lint

Validate sdt.context frontmatter and links

### Synopsis

Validate the sdt.context/ documents: frontmatter well-formed (kind, mandatory
summary), [[links]] resolve to existing files, and ADR filenames/numbers are
consistent. Exits non-zero when CRITICAL issues are found.

Examples:
  sdt context lint
  sdt context lint --format json

```
sdt context lint [flags]
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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)

