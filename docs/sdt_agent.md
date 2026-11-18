## sdt agent

Agent instruction tools (AGENTS.md, instruction files)

### Synopsis

Generate and maintain agent instruction files.

  agent init       bootstrap AGENTS.md + context/ instruction files
  agent refresh    re-render the generated surfaces without a full init

AGENTS.md carries the general agent instructions (5-stage lifecycle, knowledge
tiers, planning and work logs, communication, patterns) in a tagged
`instructions` block, plus a write-once `project` block for project-specific
stack/build/test/lint/conventions. The instruction files under
`context/instructions/` cover CLI usage plus per-type templates (analysis,
plan, tasks, decision, architecture, worklog, notes, questions, proposals, research, prompts) and the command
reference.


### Options

```
  -h, --help[=true|false]   help for agent
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

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt agent census](sdt_agent_census.md)	 - Classify agent shell calls into the adoption shapes
* [sdt agent doctor](sdt_agent_doctor.md)	 - Report the health of the SDT workspace (read-only)
* [sdt agent gate](sdt_agent_gate.md)	 - Run the delivery gate declared in .sdt.yaml
* [sdt agent init](sdt_agent_init.md)	 - Bootstrap an SDT-managed project for AI agents
* [sdt agent refresh](sdt_agent_refresh.md)	 - Re-render the generated instruction surfaces
* [sdt agent roles](sdt_agent_roles.md)	 - Role register and generated role profiles
* [sdt agent task](sdt_agent_task.md)	 - Delegate and inspect asynchronous ACP tasks
* [sdt agent verify](sdt_agent_verify.md)	 - Verify the AGENTS.md instruction contract

