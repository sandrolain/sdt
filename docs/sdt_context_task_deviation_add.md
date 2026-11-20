## sdt context task deviation add

Record a deliberate deviation from the plan

### Synopsis

Record a deliberate deviation in the task file's `## Deviations` section,
in the closed vocabulary fix | add | unblock | stop-and-ask.

The default kind is stop-and-ask: when the caller does not say why the
plan is being departed from, the recorded instruction is to ask the user. The
kind is never inferred.

With --id <checklist-id> the referenced phase item is marked rather than deleted:
`[~]` for a deviation being worked through, `[!]` for one that stopped to ask.
The item keeps its text and its id.

Examples:
  sdt context task deviation add "phase 3 check moved to phase 5" --kind fix
  sdt context task deviation add "blocked on the user's answer" --id c14 --reason "waiting on the answer"
  sdt context task deviation list

```
sdt context task deviation add "<text>" [flags]
```

### Options

```
      --agent string              Provenance: agent/tool that performed the mutation
  -h, --help[=true|false]         help for add
      --id string                 Checklist id of the phase item the deviation concerns (marked, never deleted)
      --kind string               Deviation kind: fix | add | unblock | stop-and-ask (default "stop-and-ask")
      --model string              Provenance: model id that performed the mutation
      --no-cascade[=true|false]   Do not propagate completion to parent documents
      --phase string              Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)
      --plan string               Plan reference (plan file; default: latest active plan; custom slug for standalone)
      --reason string             Why the plan was departed from (recorded with the item)
      --session string            Provenance: session id for traceability
      --stream string             Split-file label (kebab-case); selects <slug-plan>-<stream>.md
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

* [sdt context task deviation](sdt_context_task_deviation.md)	 - Record and list deliberate deviations from the plan
