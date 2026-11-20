## sdt conv

Data Conversion

### Synopsis

Data Conversion

```
sdt conv [flags]
```

### Options

```
  -h, --help[=true|false]     help for conv
  -a, --in string             Input format (json, yaml, toml, csv, msgpack)
  -o, --object[=true|false]   CSV rows as objects
  -b, --out string            Output format (json, yaml, toml, csv, msgpack)
  -s, --separator string      CSV separator (default ",")
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

* [sdt](sdt.md)	 - Smart Developer Tools
