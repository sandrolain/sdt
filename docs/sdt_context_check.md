## sdt context check

Tick a checklist item of a context/ document by its id

### Synopsis

Set the status of one checklist item of an existing context/ document
(plan, task file or questions), addressed by its id anchor ("c3" or "3") with
the positional ordinal as fallback. Un-anchored items are stamped on write.

Pass a batch selector instead of an id to update several items at once:
`--section <heading|id>` ticks the items in that section, `--phase <n>` the
items in phase <n> (its shortcut for `--section "Phase <n>"`), `--all` every
item, and `--grep <text>` the items whose text contains the substring. Pass one
selector: they are alternatives, not refinements.

Pass exactly one `--state`: done, wip, block or pending. `--state block` takes an
`--reason`. The document's `updated` is refreshed when its kind requires it.
The per-state booleans (`--done`, `--wip`, `--block`, `--pending`) still work but are
deprecated: they name a value where a flag should name a field.

Examples:
  sdt context check plan/<plan-file> c4 --state done
  sdt context check context/tasks/<task-file>.md 2 --state wip
  sdt context check plan/<plan-file> --state done --phase 3
  sdt context check plan/<plan-file> --state done --section "Validation"
  sdt context check plan/<plan-file> --state done --section completion-criteria
  sdt context check plan/<plan-file> --state done --grep "F5"
  sdt context check questions/20260927-091146-... c3 --state block --reason "needs data"

```
sdt context check <doc-ref> [id] [flags]
```

### Options

```
      --agent string              Provenance: agent/tool that performed the mutation
      --all[=true|false]          Select every checklist item (batch)
      --grep string               Select the items whose text contains this substring (batch)
  -h, --help[=true|false]         help for check
      --model string              Provenance: model id that performed the mutation
      --no-cascade[=true|false]   Do not propagate completion to parent documents
      --phase ## Phase <n>        Select the items in the ## Phase <n> section (batch; shortcut for --section "Phase <n>")
      --reason string             Reason for --block or --waive
      --section string            Select the items in the section with this heading or stable id (batch)
      --session string            Provenance: session id for traceability
      --state string              New state of the selected item(s): done|wip|block|pending
      --waive[=true|false]        Record a structured waiver (keeps the item unfinished; requires --reason)
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
