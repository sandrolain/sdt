## sdt material review

Record a reader verdict in a material's ledger

```
sdt material review <concept> --verdict <text> [flags]
```

### Options

```
      --by string           the reviewer session id
  -h, --help[=true|false]   help for review
      --lang string         language to disambiguate, e.g. it
      --model string        the model the reviewer ran on
      --role string         which cycle pass records: reader (default) or critic
      --syllabus string     syllabus to disambiguate, with or without syllabi/
      --verdict string      the reader verdict (required)
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
