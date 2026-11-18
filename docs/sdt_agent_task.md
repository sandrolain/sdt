## sdt agent task

Delegate and inspect asynchronous ACP tasks

### Synopsis

Run bounded ACP tasks in retained Git worktrees.

Example project configuration (verify each configured server's ACP behavior):

  delegation:
    max_attempts: 3
    max_duration: 45m
    agents:
      - name: reviewer-a
        command: ["your-agent", "acp"]
        model: provider/model
        roles: [reviewer]
        capabilities: [file-read, search]
        concurrency: 1
        cooldown: 10m

Token usage, provider cost, and ACP communication bytes remain unknown unless
the server reports measurable values. Results require principal acceptance or
an explicit correction; cleanup never runs automatically. Apply accepted changes
through the explicit preview and digest-checked apply commands; they never
create a commit.

Routing filters the configured agent order by an existing SDT role and every
requested capability, then enforces entry and shared-group concurrency. The
delegate receives a separate Git worktree; this is not an OS sandbox for tools
the agent starts. Permission rules are reevaluated from the live project config
at each authorization, default to ask, and pending choices are resolved by
their exact offered option id.

Automatic continuation uses structured ACP quota or provider-unavailable
signals only. Authentication, permission, unknown, and ordinary task failures
require principal action. Provider/model compatibility is specific to the
configured server version and is not inferred from ACP support alone.

### Options

```
  -h, --help[=true|false]   help for task
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
* [sdt agent task accept](sdt_agent_task_accept.md)	 -
* [sdt agent task apply](sdt_agent_task_apply.md)	 - Preview or apply accepted task changes to the principal checkout
* [sdt agent task availability](sdt_agent_task_availability.md)	 - Inspect or reset provider availability exclusions
* [sdt agent task cancel](sdt_agent_task_cancel.md)	 -
* [sdt agent task cleanup](sdt_agent_task_cleanup.md)	 - Preview or explicitly remove retained task resources
* [sdt agent task correct](sdt_agent_task_correct.md)	 -
* [sdt agent task permissions](sdt_agent_task_permissions.md)	 - Inspect or resolve ACP permission requests
* [sdt agent task result](sdt_agent_task_result.md)	 -
* [sdt agent task resume](sdt_agent_task_resume.md)	 -
* [sdt agent task start](sdt_agent_task_start.md)	 - Start a bounded delegated task
* [sdt agent task status](sdt_agent_task_status.md)	 -
