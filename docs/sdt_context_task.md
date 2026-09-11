## sdt context task

Manage per-phase task checklists

### Synopsis

Manage plan-scoped task checklists in
context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md. Each plan phase maps
to its own checklist; --phase <n> is required and --plan defaults to the
latest active plan (or an explicit --plan <plan-file> / --plan <slug> for a
standalone checklist).

  sdt context task list [--phase <n>] [--plan <ref>]      show steps with ids
  sdt context task add "<step>" --phase <n> [--plan <ref>] [--objective] [--summary]
  sdt context task done|block|wip <id> --phase <n> [--plan <ref>]
  sdt context task archive --phase <n> [--plan <ref>] [--slug]

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked

### Options

```
  -h, --help   help for task
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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
* [sdt context task add](sdt_context_task_add.md)	 - Add a step to the active task list
* [sdt context task archive](sdt_context_task_archive.md)	 - Archive the active task list to context/archive/
* [sdt context task block](sdt_context_task_block.md)	 - Mark a task step blocked
* [sdt context task done](sdt_context_task_done.md)	 - Mark a task step done
* [sdt context task list](sdt_context_task_list.md)	 - Show a per-phase task list
* [sdt context task wip](sdt_context_task_wip.md)	 - Mark a task step in progress

