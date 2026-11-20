## sdt research resume

Continue an interrupted run without repeating finished work

### Synopsis

Resume a run from its checkpoint: fetch the sources still 'discovered'
(never fetched, or a failed fetch to retry), then verify the sources that have a
payload. Work already done is skipped — a fetched or verified source is not
fetched again and not re-verified from scratch.

Replaying a completed run is idempotent: nothing new is fetched and no duplicate
source is created. The command exits non-zero when a resumed fetch fails.

Examples:
  sdt research resume
  sdt research resume --run 01a0f…

```
sdt research resume [flags]
```

### Options

```
  -h, --help[=true|false]   help for resume
      --run string          Run id (default: the latest run)
      --timeout int         Request timeout in seconds (default 60)
      --user-agent string   HTTP user agent for requests (default "sdt/1.0")
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
