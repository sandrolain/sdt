## sdt context task

Manage plan task checklists

### Synopsis

Manage plan-scoped task checklists in
context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>.md (one file per plan by default,
with one `## Phase <n>` section per phase; a legacy
<...>-<slug-plan>-phase-<n>.md file still wins, and --stream <label> selects an
opt-in split file <...>-<slug-plan>-<label>.md). --phase <n> targets a phase
section and is optional; --plan defaults to the latest active plan (or an
explicit --plan <plan-file> / --plan <slug> for a standalone checklist).

  sdt context task list [--phase <n>] [--stream <label>] [--plan <ref>]   show steps with ids
  sdt context task add "<step>" [--phase <n>] [--stream <label>] [--plan <ref>] [--summary]
  sdt context task done|block|wip <id> [--phase <n>] [--stream <label>] [--plan <ref>]
  sdt context task review [--phase <n>] [--stream <label>] [--plan <ref>]  record verdicts + complete

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked

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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
* [sdt context task add](sdt_context_task_add.md)	 - Add a step to the active task list
* [sdt context task block](sdt_context_task_block.md)	 - Mark a task step blocked
* [sdt context task claim](sdt_context_task_claim.md)	 - Claim a task file for this session
* [sdt context task deviation](sdt_context_task_deviation.md)	 - Record and list deliberate deviations from the plan
* [sdt context task done](sdt_context_task_done.md)	 - Mark a task step done
* [sdt context task list](sdt_context_task_list.md)	 - Show a task list (optionally one phase section)
* [sdt context task progress](sdt_context_task_progress.md)	 - Show a task file's per-phase progress (N/M, current phase, blocked ids)
* [sdt context task release](sdt_context_task_release.md)	 - Release a task file claim
* [sdt context task review](sdt_context_task_review.md)	 - Record the verify-step review verdicts in the phase task file
* [sdt context task wip](sdt_context_task_wip.md)	 - Mark a task step in progress
