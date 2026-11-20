## sdt q set

Set the value at a gjson path, preserving the rest of the file

### Synopsis

Set the value at a gjson path. The value is parsed as JSON when it is valid
JSON (so 3 is a number, true a bool, {"a":1} an object), otherwise it is a
plain string. <value> may be omitted when it is passed through the global
--input/--inb64/--file conventions.

Untouched bytes, key order and number literals are preserved (sjson splice).
With a real file the edit is in place; with - or piped input the result goes
to stdout.

Examples:
  sdt q set package.json 'scripts.test' 'go test ./...'
  sdt q set data.json 'items.0.done' true
  printf '%s' '{"a":1}' | sdt q set - a.b '"x"'

```
sdt q set <file> <path> <value> [flags]
```

### Options

```
  -h, --help[=true|false]   help for set
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
