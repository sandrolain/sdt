## sdt research inspect

Show a research run: sources, status, budget and checkpoint

### Synopsis

Show the recorded state of a research run without changing it. --run
selects the run; without it the latest run is shown. --format json|yaml emits
the machine-readable view.

Examples:
  sdt research inspect
  sdt research inspect --run 01a0f…  --format json

```
sdt research inspect [flags]
```

### Options

```
  -h, --help[=true|false]   help for inspect
      --run string          Run id (default: the latest run)
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

* [sdt research](sdt_research.md)	 - Auditable research runs: manifest, provenance and resume
