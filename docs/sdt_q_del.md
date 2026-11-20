## sdt q del

Delete the value at a gjson path, preserving the rest of the file

### Synopsis

Delete the value at a gjson path. A missing path is an error (exit 1). With a
real file the edit is in place; with - or piped input the result goes to
stdout.

Examples:
  sdt q del package.json 'scripts.legacy'
  cat data.json | sdt q del - 'items.0'

```
sdt q del <file> <path> [flags]
```

### Options

```
  -h, --help[=true|false]   help for del
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
