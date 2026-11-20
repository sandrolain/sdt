## sdt env set

Set or update a value in a .env file

```
sdt env set <KEY> <VALUE> [flags]
```

### Options

```
      --file string         Path to .env file (default ".env")
  -h, --help[=true|false]   help for set
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
