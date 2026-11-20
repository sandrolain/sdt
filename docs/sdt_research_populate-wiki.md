## sdt research populate-wiki

List verified sources and citations for agent-authored wiki pages

### Synopsis

List a run's verified sources with the citation each wiki page must carry
(refs/<dir>/<result>.md@<sha8>). This is a READ-ONLY brief: the CLI never writes
a context/wiki/ page. Populating the wiki is an editorial task — the agent reads
the captures, distils concepts per context/instructions/ingestion.md and writes
the pages.

Archive the captures first with `sdt research archive --apply --confirm`.
`--brief` records why the knowledge is useful to the project.

Examples:
  sdt research populate-wiki --run 01a0f…
  sdt research populate-wiki --run 01a0f… --brief "embedded vector stores" --format json

```
sdt research populate-wiki [flags]
```

### Options

```
      --brief string        Why the knowledge is useful to the project
  -h, --help[=true|false]   help for populate-wiki
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
