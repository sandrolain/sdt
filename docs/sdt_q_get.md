## sdt q get

Print the value at a gjson path

### Synopsis

Print the value at a gjson path. A scalar prints its text, an object or an
array prints its raw JSON, so stdout stays valid JSON for a nested result.

The document is read from <file>, or from the global --input/--inb64/--file
conventions when no file is given (use - to read stdin). --format json emits
an {path,value,exists,type} envelope; --default returns a value instead of
failing when the path is absent.

Examples:
  sdt q get context/index.json 'documents.0.title'
  sdt q get context/index.json 'documents.#(kind=="analysis")#.path'
  cat data.json | sdt q get 'items.#'

```
sdt q get <file> <path> [flags]
```

### Options

```
      --default string      Value to print (and exit 0) when the path is absent
  -h, --help[=true|false]   help for get
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

* [sdt q](sdt_q.md)	 - Query and patch JSON files by gjson path
