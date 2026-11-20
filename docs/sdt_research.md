## sdt research

Auditable research runs: manifest, provenance and resume

### Synopsis

Manage research runs with an auditable YAML manifest.

A run lives under context/tmp/research/<run-id>/ and records its query, budget,
checkpoint and one entry per source (canonical URL, provenance, status). The
content acquisition and verification stages are added by later commands; this
group creates a run and inspects it.

### Options

```
  -h, --help[=true|false]   help for research
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

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt research archive](sdt_research_archive.md)	 - Archive a run's verified captures into context/refs/ (gated)
* [sdt research fetch](sdt_research_fetch.md)	 - Acquire named URLs into the run with provenance
* [sdt research init](sdt_research_init.md)	 - Create a research run
* [sdt research inspect](sdt_research_inspect.md)	 - Show a research run: sources, status, budget and checkpoint
* [sdt research populate-wiki](sdt_research_populate-wiki.md)	 - List verified sources and citations for agent-authored wiki pages
* [sdt research resume](sdt_research_resume.md)	 - Continue an interrupted run without repeating finished work
* [sdt research search](sdt_research_search.md)	 - Discover sources for a query and record them in the run
* [sdt research synthesize](sdt_research_synthesize.md)	 - Assemble a cited draft from the run's verified sources
* [sdt research verify](sdt_research_verify.md)	 - Check a run's recorded facts and mark sources verified/rejected
