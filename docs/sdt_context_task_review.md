## sdt context task review

Record the verify-step review verdicts in the phase task file

### Synopsis

Append the verify-step review block to the phase task file and, when every
checklist item is done, mark the file completed.

Each finding ends as one of the closed verdicts (CONFIRMED | DISPROVED | UNVERIFIED | DISCREPANT) with
evidence; an independent pass validates findings and the phase author does not
self-approve. Use --input/--file/piped stdin for the findings prose.

Record per-finding verdicts with --finding and --verdict: the two flags pair by
position and are repeatable, and each pair becomes one item under the block's
`### Findings` subsection with a CLI-assigned id. Re-running the same pair is
a no-op, so a phase's finding identity is stable; a claim re-stated with a
different verdict is a new finding and gets a new id.

Examples:
  sdt context task review --phase 1 --plan plan.md --input "all gates green (CONFIRMED)"
  sdt context task review --phase 1 --plan plan.md \
    --finding "vet is clean" --verdict CONFIRMED \
    --finding "coverage target met" --verdict UNVERIFIED

```
sdt context task review [flags]
```

### Options

```
      --agent string              Provenance: agent/tool that performed the mutation
      --finding ### Findings      Finding claim to record under ### Findings (repeatable; pair by position with --verdict)
  -h, --help[=true|false]         help for review
      --model string              Provenance: model id that performed the mutation
      --no-cascade[=true|false]   Do not propagate completion to parent documents
      --phase string              Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)
      --plan string               Plan reference (plan file; default: latest active plan; custom slug for standalone)
      --session string            Provenance: session id for traceability
      --stream string             Split-file label (kebab-case); selects <slug-plan>-<stream>.md
      --verdict stringArray       Verdict for the matching --finding: CONFIRMED | DISPROVED | UNVERIFIED | DISCREPANT (repeatable)
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

* [sdt context task](sdt_context_task.md)	 - Manage plan task checklists
