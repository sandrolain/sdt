## sdt context close

Close a document in one step (task → plan → analysis)

### Synopsis

Set a document to its terminal `completed` status in one step and cascade the
completion to its parents. It refuses when the derived state disagrees
(unfinished children, or an unfinished task checklist) unless
`--allow-unfinished` is given together with `--reason`, which records the
override in the document body.

Closing a plan also writes its crystallized closeout in the worklog directory: the
checklist states, the `## Review` verdicts (findings included) and the gate result
are generated from the ledger, while the fields only the author knows can be
recorded in the same invocation with `--files`, `--decisions`, `--lessons`,
`--signal` and `--deviations`, so the worklog needs no hand edit. A field left
out keeps its marker; a rerun regenerates the derived part and keeps what the
author wrote.

Supported kinds: tasks, plan, analysis.

Exit: 0 closed, 1 failed, 4 unfinished — a refusal is a decision for the caller,
so it prints the open items and the two commands that resolve them instead of
reporting a failure (see 'sdt --help' for the exit vocabulary).

Examples:
  sdt context close context/tasks/<task-file>.md
  sdt context close plan/<plan-file>
  sdt context close plan/<plan-file> --allow-unfinished --reason "shipping the remainder separately"

```
sdt context close <doc-ref> [flags]
```

### Options

```
      --agent string                    Provenance: agent/tool that performed the mutation
      --allow-unfinished[=true|false]   Close despite unfinished children/tasks (requires --reason)
      --decisions string                Decisions recorded in the closeout
      --deviations string               Deviations / blockers recorded in the closeout
      --files string                    Files affected recorded in the closeout
  -h, --help[=true|false]               help for close
      --lessons string                  Lessons / Do-Not-Repeat candidates recorded in the closeout
      --model string                    Provenance: model id that performed the mutation
      --no-cascade[=true|false]         Do not propagate completion to parent documents
      --reason string                   Why the document is closed early (required with --allow-unfinished)
      --session string                  Provenance: session id for traceability
      --signal string                   Signal review recorded in the closeout
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
