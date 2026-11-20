## sdt context relations

Manage typed parent relations

### Synopsis

Manage the typed, bidirectional parent relations keyed by the document `uid`.

  sdt context relations backfill [--dry-run]   derive the plan→analysis and
                                               task→plan edges from `sources` and stamp both directions

### Options

```
  -h, --help[=true|false]   help for relations
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
* [sdt context relations backfill](sdt_context_relations_backfill.md)	 - Derive the typed parent relations from sources
* [sdt context relations edges](sdt_context_relations_edges.md)	 - List the recorded edges of a document
* [sdt context relations parent](sdt_context_relations_parent.md)	 - Print the recorded parent of a document
* [sdt context relations path](sdt_context_relations_path.md)	 - Show a document's lifecycle chain (deprecated: use `relations path`)
* [sdt context relations rank](sdt_context_relations_rank.md)	 - Rank candidate documents of a type (deprecated: use `relations rank`)
* [sdt context relations similar](sdt_context_relations_similar.md)	 - List the documents semantically nearest to a document (deprecated: use `relations similar`)
