## sdt research synthesize

Assemble a cited draft from the run's verified sources

### Synopsis

Assemble a deterministic, cited Markdown draft from the run's verified
sources: an outline plus one excerpt per source with a hash-anchored citation.
No model and no network are involved.

Unverified sources are excluded unless --include-unverified, in which case they
appear marked UNVERIFIED. Sources without a payload and rejected ones are listed
under "Skipped". The draft is written only to the target you name — a transient
path under context/tmp/ or a draft document; it never promotes anything into
approved knowledge.

Examples:
  sdt research synthesize --out context/tmp/draft.md
  sdt research synthesize --include-unverified --max-lines 40 --out draft.md

```
sdt research synthesize [flags]
```

### Options

```
  -h, --help[=true|false]                 help for synthesize
      --include-unverified[=true|false]   Include unverified sources, marked as such
      --max-lines int                     Cap the excerpt per source (0 = all)
      --out string                        Write the draft here (default: stdout)
      --run string                        Run id (default: the latest run)
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
