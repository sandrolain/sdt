## sdt context todo

Manage the idea inbox (short annotate-only TODOs)

### Synopsis

Manage context/todo.yaml: short, annotate-only ideas for future analyses.

  sdt context todo add "<text>" [--id <slug>] [--source <ref>] [--tag <tag>...]
  sdt context todo list [--undone | --done] [--tag <tag>]
  sdt context todo done <id> | --all
  sdt context todo remove <id>...
  sdt context todo prune --source <ref>

Adding only annotates: it never starts an analysis, a plan or any operation.
Every mutation regenerates the human-readable projection context/todo.md.

### Options

```
  -h, --help[=true|false]   help for todo
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
* [sdt context todo add](sdt_context_todo_add.md)	 - Add a short idea
* [sdt context todo done](sdt_context_todo_done.md)	 - Mark an idea done (kept in the list)
* [sdt context todo list](sdt_context_todo_list.md)	 - List the inbox items
* [sdt context todo prune](sdt_context_todo_prune.md)	 - Remove every idea that came from a source
* [sdt context todo remove](sdt_context_todo_remove.md)	 - Remove one or more ideas from the inbox
