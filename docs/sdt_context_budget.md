## sdt context budget

Report the measured size of the always-loaded context/ surface

### Synopsis

Report the measured byte, line and word cost of the surface a session
always loads: the AGENTS.md instructions block and its write-once project block
(the two together are the whole every-turn file), context/index.md, and the two
essential-tier directories context/architecture/ and context/decisions/. Each
surface is reported separately with its largest entries.

WHEN — you are deciding what the corpus costs to load, or judging whether a
proposed change makes the always-loaded surface bigger, and want a number rather
than an impression.

WHEN NOT — you want the cost of one document (sdt context show, or wc on the
file); the per-type instruction modules, which load on demand (pass
--include-modules to measure them anyway); or a model-accurate token count. Any
token figure here is a documented estimate derived from bytes, not a tokenizer
result.

RETURNS — bytes, lines and words per surface and per file, the n largest
entries per surface under --top, the totals, and an explicitly labelled token
estimate. Read-only: the report never writes a file, never exits non-zero
because a surface is large, and is never a gate.

EXAMPLE — sdt context budget
          sdt context budget --top 3 --format json
          sdt context budget --include-modules --bytes-per-token 3

```
sdt context budget [flags]
```

### Options

```
      --bytes-per-token int            Divisor behind the estimated-token figure (a documented estimate, not a tokenizer count) (default 4)
  -h, --help[=true|false]              help for budget
      --include-modules[=true|false]   Also measure the per-type instruction modules, which load on demand
      --top int                        Largest entries listed per surface (0 lists none) (default 10)
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
