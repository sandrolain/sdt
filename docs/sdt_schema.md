## sdt schema

Generate JSON Schema for SDT commands

### Synopsis

Generate JSON Schema documents describing SDT command inputs and flags.

Without a command, emits a schema for every command as a JSON array. Name one
command to get only its schema: `sdt schema jwt parse` or the equivalent
`--command "jwt parse"`. The two forms are alternatives; passing both is refused.

Examples:
  sdt schema                          # all commands
  sdt schema jwt parse                # single command
  sdt schema --command "jwt parse"    # the same, as one flag
  sdt schema --format yaml            # YAML output

```
sdt schema [flags]
```

### Options

```
      --command string      Command path to generate schema for (e.g. "jwt parse")
  -h, --help[=true|false]   help for schema
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
