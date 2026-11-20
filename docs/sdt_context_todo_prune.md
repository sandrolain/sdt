## sdt context todo prune

Remove every idea that came from a source

```
sdt context todo prune --source <ref> [flags]
```

### Options

```
  -h, --help[=true|false]   help for prune
      --source string       Remove every idea with this source
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

* [sdt context todo](sdt_context_todo.md)	 - Manage the idea inbox (short annotate-only TODOs)
