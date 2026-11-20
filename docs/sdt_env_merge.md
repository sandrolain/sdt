## sdt env merge

Merge multiple .env files (last value wins)

```
sdt env merge [flags]
```

### Options

```
      --files string        Comma-separated list of .env files to merge
  -h, --help[=true|false]   help for merge
      --output string       Write merged output to file (default: print JSON)
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

* [sdt env](sdt_env.md)	 - Parse and manipulate .env files
