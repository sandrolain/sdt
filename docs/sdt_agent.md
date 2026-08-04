## sdt agent

Agent instruction tools (AGENTS.md, sections, guides)

### Synopsis

Generate and maintain agent instruction files.

  agent init       generate an opinionated AGENTS.md
  agent section    integrate/update tagged sections inside AGENTS.md
  agent guide      generate an extended skill guide

Sections are delimited by start/end tags that make them safe to update
individually without touching the rest of the file:

  <!-- sdt:begin:NAME -->
  ...content...
  <!-- sdt:end:NAME -->

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
* [sdt agent guide](sdt_agent_guide.md)	 - Generate an extended SDT skill guide
* [sdt agent init](sdt_agent_init.md)	 - Bootstrap an SDT-managed project for AI agents
* [sdt agent section](sdt_agent_section.md)	 - Manage tagged sections in AGENTS.md

