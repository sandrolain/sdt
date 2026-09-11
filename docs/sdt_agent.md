## sdt agent

Agent instruction tools (AGENTS.md, instruction files)

### Synopsis

Generate and maintain agent instruction files.

  agent init       bootstrap AGENTS.md + context/ instruction files

AGENTS.md carries the general agent instructions (5-phase lifecycle, knowledge
tiers, planning and work logs, communication, patterns) in a tagged
`instructions` block, plus a write-once `project` block for project-specific
stack/build/test/lint/conventions. The instruction files under
`context/instructions/` cover CLI usage plus per-type templates (analysis,
plan, tasks, adr, architecture, worklog, notes, questions) and the command
reference.


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
* [sdt agent verify](sdt_agent_verify.md)	 - Verify the AGENTS.md instruction contract

