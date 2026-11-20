## sdt context map

Map the corpus: what it holds, what is awkward, and what it cannot see

### Synopsis

Map the corpus and state the map's own scope.

WHEN — you meet a corpus cold: this is the one page that says which kinds exist,
which frontmatter keys they share, which objectives are alive, which documents
nobody can trace, and what is known to be awkward (dead ends, postponed
analyses with their revival condition, open questions, lint counts).

WHEN NOT — you know what you are looking for: map/search the specific thing.

The first line is the scope, in every format. `context/refs`, `context/tmp`,
`context/trash` and `context/sdtdocs` are outside the map, so an empty section never
means the thing is absent — it means the thing is not a corpus document.

Reads the documents, never `context/index.json`: a stale index must not produce a
confident wrong answer. No cache, no write.

Example: `sdt context map` · `sdt context map --objective agent-refs --format json`

```
sdt context map [flags]
```

### Options

```
  -h, --help[=true|false]   help for map
      --objective string    Limit the map to one objective slug
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
