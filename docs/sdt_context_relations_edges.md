## sdt context relations edges

List the recorded edges of a document

### Synopsis

List the edges a document records — outbound by default — from the one edge
model every surface reads: `sources` (derivation), `links` (correlation), the wiki
frontmatter `relations`, typed body links and the lifecycle parent.

A target the corpus does not serve is not an edge: a `refs/` reference, an excluded
path and a wiki relation that does not resolve are left out.

The query answers one hop. `--depth` accepts 1; transitive expansion is a
separate wave and is refused rather than silently not done.

Examples:
  sdt context relations edges plan/<plan-file>
  sdt context relations edges wiki/architecture/deployment-pipeline --inbound
  sdt context relations edges analysis/<file> --verb depends_on --format json

```
sdt context relations edges <doc-ref> [flags]
```

### Options

```
      --depth int              Hops to follow; only 1 is answered (default 1)
  -h, --help[=true|false]      help for edges
      --inbound[=true|false]   List the edges arriving at the document instead of the ones leaving it
      --verb string            Only edges carrying this relation verb
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

* [sdt context relations](sdt_context_relations.md)	 - Manage typed parent relations
