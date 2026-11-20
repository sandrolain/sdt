## sdt agent census

Classify agent shell calls into the adoption shapes

### Synopsis

Read one JSON object per line — {"ts": "<RFC3339>", "command": "<shell>"} —
classify each command into the adoption shapes, and print the counts and shares.

WHEN — re-running the fallback-shape measurement, or comparing a window before and
after a verb shipped. The input comes from an extractor script, not from a
database this command knows about.

RETURNS — a table by default, a {window, total, shapes} object with --format json.
A malformed line is refused with its number; unknown fields are ignored.

EXAMPLE — sdt agent census --file calls.jsonl --since 2026-10-01
          sdt agent census --file - --until 2026-10-24 --format json

```
sdt agent census [flags]
```

### Options

```
  -f, --file string         JSONL extract (default: stdin; - for stdin) (default "-")
  -h, --help[=true|false]   help for census
      --since string        Include calls on/after this RFC3339 instant or date
      --until string        Include calls on/before this RFC3339 instant or date
```

### Options inherited from parent commands

```
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
