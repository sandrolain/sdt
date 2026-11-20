## sdt research verify

Check a run's recorded facts and mark sources verified/rejected

### Synopsis

Verify the recorded facts of a run, offline and deterministically: it
recomputes the SHA-256 from each source's raw payload, checks the payload exists
and is non-empty, validates the manifest schema and marks each source
'verified' or 'rejected' with a reason. It never promotes a source it cannot
check.

Sources still 'discovered' (never fetched) are left untouched. The command exits
non-zero when any checked source is rejected — recorded, not enforcing.

Examples:
  sdt research verify
  sdt research verify --run 01a0f…  --format json

```
sdt research verify [flags]
```

### Options

```
  -h, --help[=true|false]   help for verify
      --run string          Run id (default: the latest run)
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
