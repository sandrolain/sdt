## sdt config

Configuration Tools

### Synopsis

Configuration Tools

### Options

```
  -h, --help[=true|false]   help for config
  -k, --key string          Flag Key Path
  -t, --type string         Value Type (s[tring], i[nt], f[loat], j[son]) (default "json")
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
* [sdt config get](sdt_config_get.md)	 - Get Configuration Value
* [sdt config init](sdt_config_init.md)	 - Initialize .sdt.yaml with project identity
* [sdt config set](sdt_config_set.md)	 - Set Configuration Value
* [sdt config show](sdt_config_show.md)	 - Show project configuration
