## sdt diff

Compare two files and output differences

### Synopsis

Compare two files. Supported output formats:
  unified    — standard unified diff (default)
  json-patch — RFC 6902-style JSON patch operations (both inputs must be JSON)

```
sdt diff [flags]
```

### Options

```
      --a string             Path to first file
      --b string             Path to second file
      --context int          Lines of context around changes (unified diff only) (default 3)
      --diff-format string   Output format: unified or json-patch (default "unified")
  -h, --help[=true|false]    help for diff
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
