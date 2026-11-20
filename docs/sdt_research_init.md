## sdt research init

Create a research run

### Synopsis

Create a new research run under context/tmp/research/<run-id>/ with its
manifest and provenance sidecar. --query is the question the run answers;
--scope narrows it; --objective is the kebab-case grouping key that names the
run's archive directory (derived from the query when omitted). The run id is
printed (or the manifest in --format json|yaml).

Examples:
  sdt research init --query "which embedded vector store fits SDT"
  sdt research init --query "crawldown options" --scope "capture flags only"
  sdt research init --query "capture layout" --objective web-capture-tooling

```
sdt research init [flags]
```

### Options

```
      --from [title](url)   Seed the run from a list of known sources (one URL per line, or [title](url))
  -h, --help[=true|false]   help for init
      --objective string    Kebab-case grouping key naming the archive directory (default: derived from the query)
      --query string        The question the run answers
      --scope string        Optional scope narrowing the run
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
