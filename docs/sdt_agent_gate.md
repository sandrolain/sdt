## sdt agent gate

Run the delivery gate declared in .sdt.yaml

### Synopsis

Run the project's own delivery gate: the commands declared under `gate:` in
.sdt.yaml, in order, stopping at the first failure (fail-closed). This is the
strict, explicit counterpart of the advisory verify-step.

JSON/YAML reports use schema_version 1, identify revision and configuration, and
include every declared step. A step with an exclude reason is reported as excluded;
steps after a failure are not-run. Neither outcome is counted as a passed check.

Each step's `run` is a shell command line executed with /bin/sh -c in the
project root, so a step can expand its own package list. Nothing is assumed about
the toolchain: with no `gate:` declared the command reports "no gate declared"
and exits 0, and a missing tool surfaces as that step failing.

A failing step's console block is the tail of its output — the head is usually the
echo of the command — and the whole output is written to
context/tmp/gate-<step>-<timestamp>.log, named in the block and in the structured
shape's `log` field. The console stays short; the evidence survives.

With --record the run is appended to the task file's `## Review` block as a
`### Gate` subsection, one line per step with its real status and duration.
The record is written whether the run passed or failed, and a failing run is
never summarised as a pass. Without --record the gate writes nothing (the CI
path is unchanged).

Examples:
  sdt agent gate
  sdt agent gate --record --plan 20261005-example.md
  sdt agent gate --format json

```
sdt agent gate [flags]
```

### Options

```
  -h, --help[=true|false]     help for gate
      --phase string          Plan phase whose task file receives the record (default: the plan's whole-plan file)
      --plan string           Plan reference whose task file receives the record (required by --record)
      --record[=true|false]   Append the run to the task file's ## Review block (requires --plan; without it the gate writes nothing)
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)
