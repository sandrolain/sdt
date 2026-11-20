## sdt manifest

Emit a machine-readable manifest of all available commands

### Synopsis

Emit a JSON or YAML manifest of the full command tree.

Each root command includes one of the purpose categories shown in root help.
Useful for AI agents to auto-discover capabilities without parsing help text.

```
sdt manifest [flags]
```

### Options

```
  -h, --help[=true|false]   help for manifest
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
