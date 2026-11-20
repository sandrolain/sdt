## sdt research archive

Archive a run's verified captures into context/refs/ (gated)

### Synopsis

Archive the verified captures of a research run into the run's dated,
objective-scoped directory under context/refs/. The default is a PREVIEW that
writes nothing.

--apply performs the archive, but only when you also pass --confirm: it copies
each verified capture, records the run's archive directory and prints the
citation pack (refs/<dir>/<result>.md@<sha8>) the wiki page cites. A source SDT
could not verify is never archived; refused sources are listed with the reason.
No wiki page is written: populating the wiki is an agent editorial task.

Examples:
  sdt research archive --run 01a0f…
  sdt research archive --run 01a0f… --format json
  sdt research archive --run 01a0f… --apply --confirm

```
sdt research archive [flags]
```

### Options

```
      --apply[=true|false]     Apply the archive (default: preview only)
      --confirm[=true|false]   Confirm the write into context/refs/ (required with --apply)
  -h, --help[=true|false]      help for archive
      --run string             Run id (default: the latest run)
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

* [sdt research](sdt_research.md)	 - Auditable research runs: manifest, provenance and resume
