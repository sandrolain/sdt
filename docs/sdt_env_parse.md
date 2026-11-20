## sdt env parse

Parse a .env file and output as JSON or shell exports

```
sdt env parse [flags]
```

### Options

```
      --file string         Path to .env file (reads stdin if not set)
  -h, --help[=true|false]   help for parse
```

### Options inherited from parent commands

```
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

* [sdt env](sdt_env.md)	 - Parse and manipulate .env files
