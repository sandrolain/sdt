## sdt schema

Generate JSON Schema for SDT commands

### Synopsis

Generate JSON Schema documents describing SDT command inputs and flags.

Without --command, emits a schema for every command as a JSON array.
With --command, emits the schema for a single command.

Examples:
  sdt schema                          # all commands
  sdt schema --command "jwt parse"    # single command
  sdt schema --format yaml            # YAML output

```
sdt schema [flags]
```

### Options

```
      --command string   Command path to generate schema for (e.g. "jwt parse")
  -h, --help             help for schema
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools

