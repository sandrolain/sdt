## sdt config init

Initialize .sdt.yaml with project identity

### Synopsis

Create a .sdt.yaml file in the current directory.

The file stores the project identity used by project-scoped commands:
  project     — project name
  group       — group name

Examples:
  sdt config init --project myapp
  sdt config init --project myapp --group platform

```
sdt config init [flags]
```

### Options

```
      --force            Overwrite existing .sdt.yaml
      --group string     Group name
  -h, --help             help for init
      --project string   Project name
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

