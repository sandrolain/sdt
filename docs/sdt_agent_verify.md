## sdt agent verify

Verify the AGENTS.md instruction contract

### Synopsis

Validate that the AGENTS.md instruction contract matches the generated
instruction files:

  - AGENTS.md exists and carries the instructions block
  - every generated instruction file exists in context/instructions/
  - no generated instruction file opens with an H1 title (WARNING)
  - no obsolete instruction files linger
  - the context/ working directories exist (including context/scripts/)
  - the block references every generated instruction file

Exits non-zero when CRITICAL issues are found.

Examples:
  sdt agent verify
  sdt agent verify --format json

```
sdt agent verify [flags]
```

### Options

```
  -h, --help[=true|false]   help for verify
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
