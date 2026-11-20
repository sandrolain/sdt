## sdt research search

Discover sources for a query and record them in the run

### Synopsis

Discover sources for a query with the project's declared providers, tried in
the order .sdt.yaml gives them, and record them as 'discovered' in the run
manifest. Search is opt-in: it refuses to run without --network and --confirm,
because it reaches the network and consumes provider credits.

Only discovery happens here; content acquisition stays with 'sdt crawldown'.

No research: block means tavily, firecrawl, exa, searxng. A paid provider whose
key is not set is skipped and recorded; SearXNG needs a declared endpoint. Keys
come from the entry (key, key_env, key_file), then the shell or the project .env.
--api-key/--api-url add an ad-hoc Firecrawl entry before the declared order.
--limit caps the results and --max-credits aborts before starting when the first
provider's remaining credits are below it.

Examples:
  sdt research init --query "vector stores"
  sdt research search --query "embedded vector store Go" --limit 5 --network --confirm
  sdt research search --run 01a0f… --query "…" --max-credits 10 --network --confirm

```
sdt research search [flags]
```

### Options

```
      --api-key string         Firecrawl API key (default: FIRECRAWL_API_KEY)
      --api-url string         Firecrawl API base URL (default: hosted endpoint)
      --confirm[=true|false]   Confirm the run may spend provider credits
  -h, --help[=true|false]      help for search
      --limit int              Maximum number of results (default 10)
      --max-credits int        Abort when remaining credits are below this (0 = no check)
      --network[=true|false]   Allow the command to reach the network
      --query string           Search query (default: the run query)
      --run string             Run id (default: the latest run)
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
