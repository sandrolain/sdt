## sdt agent verify

Verify the AGENTS.md instruction contract

### Synopsis

Validate that the AGENTS.md instruction contract matches the generated
instruction files:

  - AGENTS.md exists and carries the instructions block
  - every generated instruction file exists in context/instructions/
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
  -h, --help   help for verify
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)

