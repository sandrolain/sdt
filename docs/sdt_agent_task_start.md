## sdt agent task start

Start a bounded delegated task

### Synopsis

Create a retained worktree and start an asynchronous ACP task.

The project must declare delegation.agents, max_attempts, and max_duration in
.sdt.yaml. Candidates are tried in declaration order after role/capability
filtering. Example: sdt agent task start --role reviewer --capability file-read --brief "Review this change".

```
sdt agent task start [flags]
```

### Options

```
      --brief string         Task objective and acceptance criteria
      --capability strings   Indispensable capability (repeatable)
      --commit string        Identified source commit (default "HEAD")
  -h, --help[=true|false]    help for start
      --id string            Task id (generated when omitted)
      --include strings      Explicit project-relative dirty file to include (repeatable)
      --reference strings    Project-relative reference file to copy into the stable task workspace (repeatable)
      --role string          Existing SDT role for the task
      --worktree string      Use an existing dedicated task worktree
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

* [sdt agent task](sdt_agent_task.md)	 - Delegate and inspect asynchronous ACP tasks
