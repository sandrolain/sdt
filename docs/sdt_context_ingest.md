## sdt context ingest

Track sources in context/ingestion/ (list, status, archive, plan)

### Synopsis

Track the raw sources that wait in context/ingestion/.

A binary source cannot carry frontmatter, so the pending marker is the derived
manifest (context/ingestion/manifest.yaml), not a YAML block inside the EPUB.
The commands never convert or distil:

  list    scan context/ingestion/ and derive pending/converted/archived
  status  print one source's derived entry
  archive move a source to context/refs/ after a hash check
  plan    print the source, route, counts, scope and merge|reset mode

### Options

```
  -h, --help[=true|false]   help for ingest
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
* [sdt context ingest archive](sdt_context_ingest_archive.md)	 - Move an ingestion source to context/refs/ after a hash check
* [sdt context ingest list](sdt_context_ingest_list.md)	 - List context/ingestion/ sources with a derived status
* [sdt context ingest plan](sdt_context_ingest_plan.md)	 - Print the source, route, counts, scope and merge|reset mode
* [sdt context ingest status](sdt_context_ingest_status.md)	 - Print one ingestion source's derived status
