## sdt context ingest list

List context/ingestion/ sources with a derived status

```
sdt context ingest list [flags]
```

### Options

```
  -h, --help[=true|false]    help for list
      --write[=true|false]   Write the derived manifest to context/ingestion/manifest.yaml
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
