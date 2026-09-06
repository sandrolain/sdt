## sdt context

Context Tools (sdt.context/ work files)

### Synopsis

Manage the agent working files under sdt.context/: plans, work logs, notes
and the active task list.

  sdt context path [--type ...]   print a work file path
  sdt context new --type ...      create a work file with frontmatter
  sdt context list --type ...     list existing work files
  sdt context task ...            manage the active task list

### Options

```
  -h, --help   help for context
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
* [sdt context docs](sdt_context_docs.md)	 - Generate agent docs in sdt.context/docs/
* [sdt context lint](sdt_context_lint.md)	 - Validate sdt.context frontmatter and links
* [sdt context list](sdt_context_list.md)	 - List sdt.context/ work files
* [sdt context new](sdt_context_new.md)	 - Create a sdt.context/ work file with frontmatter
* [sdt context path](sdt_context_path.md)	 - Print the path for a sdt.context/ work file
* [sdt context reindex](sdt_context_reindex.md)	 - Regenerate sdt.context/index.md from frontmatter summaries
* [sdt context status](sdt_context_status.md)	 - Summarize sdt.context/ documents per type with next step
* [sdt context task](sdt_context_task.md)	 - Manage per-phase task checklists (sdt.context/tasks/<phase>.md)
* [sdt context template](sdt_context_template.md)	 - Print the per-type instruction file for a context type

