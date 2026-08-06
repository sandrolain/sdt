## sdt context docs

Generate agent docs in sdt.context/docs/

### Synopsis

Generate a per-command reference under sdt.context/docs/ (gitignored) for AI
agents: a README index plus one markdown file per leaf command. The output is
regenerated from the command tree and tagged with the binary version so agents
can detect and refresh stale docs.

The directory is generated, not edited. Use --clean to remove generated files
that no longer correspond to a command.

Examples:
  sdt context docs
  sdt context docs --clean
  sdt context docs --format json

```
sdt context docs [flags]
```

### Options

```
      --clean        Remove generated files not part of this run
  -h, --help         help for docs
      --out string   Output directory (default "sdt.context/docs")
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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)

