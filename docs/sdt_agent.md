## sdt agent

Agent instruction tools (AGENTS.md, instruction files)

### Synopsis

Generate and maintain agent instruction files.

  agent init       bootstrap AGENTS.md + sdt.context/ instruction files

AGENTS.md is a thin index: it carries a single tagged block that points to the
instruction files under `sdt.context/instructions/`. Each instruction
file is the single source of truth for one topic.

### Options

```
  -h, --help   help for agent
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
* [sdt agent init](sdt_agent_init.md)	 - Bootstrap an SDT-managed project for AI agents

