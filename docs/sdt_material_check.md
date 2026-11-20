## sdt material check

Refuse a package that still carries the authoring machinery

### Synopsis

Walk a file or directory and report every authoring residue in the Markdown:
an HTML comment or header, a claim map, a "Verifica" line naming the item bank,
a backticked page id, or a curator-register phrase addressing the corpus instead
of the reader. Exit 0 when clean, 1 with the findings otherwise.

```
sdt material check <path> [flags]
```

### Options

```
  -h, --help[=true|false]   help for check
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

* [sdt material](sdt_material.md)	 - Export a study guide without its authoring machinery
