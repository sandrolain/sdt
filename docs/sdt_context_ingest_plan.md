## sdt context ingest plan

Print the source, route, counts, scope and merge|reset mode

### Synopsis

Print the run plan for one ingestion source.

The command is read-only unless --write is set. It derives the source status;
with --artifact it reads the converted artifact and prints the routing counts.
Coverage counters need the pages' citations and stay "not audited" here: run
sdt context wiki coverage (or sdt context wiki audit) for the countable block.
--scope records the bounded units the run will read; --mode records whether the
run merges into the corpus or resets the source's pages first. --write persists
the same plan under context/ingestion/runs/<UTC timestamp>.yaml and never edits
the source or a page.

```
sdt context ingest plan <source> [flags]
```

### Options

```
      --artifact string      Converted artifact to measure (optional)
  -h, --help[=true|false]    help for plan
      --inventory string     Concept-inventory note to name in the plan (optional)
      --mode string          merge|reset (default "merge")
      --scope string         Bounded units the run will read (default "all")
      --write[=true|false]   Persist the plan under context/ingestion/runs/
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

* [sdt context ingest](sdt_context_ingest.md)	 - Track sources in context/ingestion/ (list, status, archive, plan)
