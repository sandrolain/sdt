## sdt config show

Show project configuration

### Synopsis

Print the project configuration resolved from .sdt.yaml found by walking
up from the current directory (like .git).

```
sdt config show [flags]
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
  -k, --key string          Flag Key Path
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
  -t, --type string         Value Type (s[tring], i[nt], f[loat], j[son]) (default "json")
```

### SEE ALSO

* [sdt config](sdt_config.md)	 - Configuration Tools

