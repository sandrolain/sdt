## sdt context wiki receipt lint

Check a note's fenced receipt block for the required counters

### Synopsis

Check every fenced receipt block in a note.

A receipt block starts with "source:" and "route:" and must carry the routing
count, the coverage denominator, the four coverage counters and the six concept
counters:

  source: <refs/...>
  route: single-pass|book-scale
  route_headings: <n>
  lines: <n>
  coverage_units: <n>
  coverage: ok=<n> part=<n> gap=<n> no_op=<n>
  concepts: candidates=<n> paged=<n> merged=<n> inline=<n> declined=<n> pending=<n>

A missing line is a finding and exits 1. The command reads the note; it never
writes it.

```
sdt context wiki receipt lint <note> [flags]
```

### Options

```
  -h, --help[=true|false]   help for lint
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

* [sdt context wiki receipt](sdt_context_wiki_receipt.md)	 - Receipt operations (lint a note's receipt block)
