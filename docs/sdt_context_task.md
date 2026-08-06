## sdt context task

Manage the active task list (sdt.context/tasks/TODO.md)

### Synopsis

Manage the active task list in sdt.context/tasks/TODO.md.

  sdt context task list                        show steps with ids
  sdt context task add "<step>" [--objective]  add a step (creates the list)
  sdt context task done|block|wip <id>         update a step status
  sdt context task archive [--slug]            archive the list and start fresh

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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)
* [sdt context task add](sdt_context_task_add.md)	 - Add a step to the active task list
* [sdt context task archive](sdt_context_task_archive.md)	 - Archive the active task list to sdt.context/archive/
* [sdt context task block](sdt_context_task_block.md)	 - Mark a task step blocked
* [sdt context task done](sdt_context_task_done.md)	 - Mark a task step done
* [sdt context task list](sdt_context_task_list.md)	 - Show the active task list
* [sdt context task wip](sdt_context_task_wip.md)	 - Mark a task step in progress

