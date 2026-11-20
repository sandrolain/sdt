## sdt agent doctor

Report the health of the SDT workspace (read-only)

### Synopsis

Run a read-only health check over the SDT-managed workspace: AGENTS.md
and its instruction block, the generated instruction files (missing/obsolete),
the working directories, the .sdt.yaml project identity, the declared delivery
gate, the config file, the derived search cache and the corpus exclusions.

Unlike 'sdt agent verify' (which fails on CRITICAL issues), doctor always exits
0 and prints a per-check report with a remediation hint.

Examples:
  sdt agent doctor
  sdt agent doctor --format json

```
sdt agent doctor [flags]
```

### Options

```
  -h, --help[=true|false]   help for doctor
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)
