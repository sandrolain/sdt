## sdt research fetch

Acquire named URLs into the run with provenance

### Synopsis

Fetch explicitly named URLs (or every 'discovered' source when no URL is
given) into the run with 'sdt crawldown's pipeline: each page is converted to
Markdown, written under the run's raw/ directory, and recorded in the manifest
with its title, date, content type, size, SHA-256 and a checkpoint.

Only the URLs you name (or the discovered sources) are fetched — this is not a
recursive crawler. A failed URL is recorded with its error and a retry count,
and does not abort the others; the command exits non-zero when any failed.

Examples:
  sdt research fetch https://example.com/a https://example.com/b
  sdt research fetch            # fetch every 'discovered' source recorded by search

```
sdt research fetch <url>... [flags]
```

### Options

```
      --delay int           Delay in seconds between fetches
  -h, --help[=true|false]   help for fetch
      --run string          Run id (default: the latest run)
      --timeout int         Request timeout in seconds (default 60)
      --user-agent string   HTTP user agent for requests (default "sdt/1.0")
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
