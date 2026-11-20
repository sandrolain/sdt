## sdt context search

Search the context/ corpus (section-level, ranked)

### Synopsis

Search the context/ knowledge base and print compact ranked results.

The index is section-level and derived from context/*.md (never a source of
truth); the scan is incremental via .sdt/cache/manifest.json. Filters narrow the
result set; by default superseded/archived documents are excluded unless --all.

Examples:
  sdt context search "hybrid search"
  sdt context search "dead-end" --type analysis --status active
  sdt context search "ranking" --topic search --limit 5 --format json
  sdt context search "filter" --date updated --last 7d --where objective=my-group

Filters (all AND-ed; shared by list and search):
  --status <s>             frontmatter status
  --after | --before <when>  dated on/after | on/before
  --last <N><unit>         window: m=minutes, h=hours, d=days, w=weeks
  --date created|updated   reference timestamp (default created)
  --where <k=v>|<k!=v>     any frontmatter field (repeatable)
  --since | --until        aliases of --after | --before

<when> accepts YYYY-MM-DD, RFC3339, now and now-<N><unit>. --where matches a
scalar field exactly (case-sensitive), a list field by membership, accepts the
literals true/false, and matches an absent key only for k!=v.

```
sdt context search <query> [flags]
```

### Options

```
      --after string            Dated on/after (YYYY-MM-DD, RFC3339, now, now-<N><unit>)
      --all[=true|false]        Include superseded/archived documents
      --before string           Dated on/before (same forms as --after)
      --category string         Filter by frontmatter category
      --date string             Reference timestamp: created (default) or updated
  -h, --help[=true|false]       help for search
      --last string             Dated in the last window, e.g. 6h, 7d, 3w (exclusive with --after)
      --limit int               Maximum results (1-100) (default 10)
      --objective string        Filter by frontmatter objective
      --semantic[=true|false]   Fuse lexical with semantic (embeddings) via RRF; degrades to lexical if unavailable
      --semantic-model string   Embedding model for --semantic (default BASE8M)
      --since string            Alias of --after
      --status string           Filter by frontmatter status (default: active; use --all for any)
      --topic string            Filter by controlled topic
      --type string             Filter by frontmatter kind
      --until string            Alias of --before
      --where stringArray       Generic frontmatter filter key=value or key!=value (repeatable; AND-ed)
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
