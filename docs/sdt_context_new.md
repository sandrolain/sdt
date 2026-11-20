## sdt context new

Create a context/ work file with frontmatter

### Synopsis

Create a plan, analysis, worklog, notes, questions, proposal, prompt,
research, architecture, decision, wiki, lesson, assessment, theme or syllabus
file under context/ with the correct naming and the full per-type YAML
frontmatter (kind, summary, context, status, created, updated, project plus
per-type fields). The body comes from --input/--file or piped stdin. Existing
files are preserved unless --force is set; --edit opens the file in $EDITOR
after creation.

The slug is derived from --title when --slug is omitted; --summary is optional
and falls back to a MANDATORY-fill placeholder so the file passes lint. For
decision type the next NNNN number is auto-assigned (override with --number).
--objective attaches a kebab-case grouping key (analysis, notes or plan; on a
notes entry it ties a dead-end to its objective); a plan defaults its objective
from the analysis named by --source. --status sets the initial lifecycle state,
validated against the type's vocabulary (the type's default when omitted; a type
without a status rejects the flag). --source records a derivation reference in
`sources` and `links` (repeatable). Wiki pages
accept subpath ids (`--slug backend/auth`) and carry `id` equal to that
subpath; a lesson does the same, carries `concept` equal to its id and defaults
`--source` to `wiki/<id>.md`. A theme and a syllabus take a subpath slug too
and keep the bare id (`--slug themes/delivery-metrics` lands at
`context/learning/themes/delivery-metrics.theme.md` with id
`themes/delivery-metrics`); the suffix names the level on the file, the directory
already names it. --agent and --role record who produced a
notes/worklog entry;
--note-type sets the notes subtype (e.g. `dead-end`). The
command prints the created file path (--format text|json|yaml).

Examples:
  sdt context new --type worklog --title "review deps" --input "reviewed deps"
  sdt context new --type plan --title "ship memory" --force
  sdt context new --type analysis --title "memory backend" --objective memory --input "..."
  sdt context new --type analysis --title "to investigate" --status draft
  sdt context new --type architecture --title "config loading" --summary "config loading component"
  sdt context new --type decision --title "Auth choice" --summary "Use JWT for auth"
	sdt context new --type questions --title "open api questions"
	sdt context new --type proposal --title "add prompt provenance"
	sdt context new --type prompt --title "deepsearch prompt"
	sdt context new --type research --title "deepsearch vector backends"

```
sdt context new [flags]
```

### Options

```
      --agent string                   Provenance: agent/tool that produced the entry (notes/worklog)
      --batch string                   Create every document in a YAML list (one entry per page)
      --category stringArray           Analysis category slug from context/categories.yaml (repeatable)
      --context string                 What triggered this entry
      --dry-run[=true|false]           With --batch, validate and print the paths without writing
      --edit[=true|false]              Open the file in $EDITOR after creation
      --entity stringArray             Entity slug the document mentions (repeatable)
      --force[=true|false]             Overwrite existing file
  -h, --help[=true|false]              help for new
      --note-type string               Notes subtype, e.g. dead-end (notes only)
      --number string                  Override for the decision number (default: next NNNN from decisions/)
      --objective string               Group key (kebab-case slug; analysis, notes or plan)
      --prior-art[=true|false]         Analysis: search and propose related documents in a Prior art section
      --prior-art-links[=true|false]   Analysis: also pre-fill links with the top prior-art candidates
      --role string                    Provenance: role that produced the entry (notes/worklog)
      --semantic[=true|false]          Prior-art: use the semantic (embeddings) branch; degrades to lexical when unavailable
      --semantic-model string          Embedding model for --semantic (default BASE8M)
      --slug string                    Slug (sanitized; overrides --title-derived slug)
      --source sources                 Source reference(s) written to sources and `links` (repeatable; a plan defaults its objective from the first analysis source)
      --status string                  Initial status (validated against the type's vocabulary; default: the type's default)
      --summary string                 Summary for the frontmatter (default: MANDATORY-fill placeholder)
      --title string                   Title (slug derived from it when --slug omitted)
      --todo todos                     Register id(s) this document answers, written to todos (repeatable; a plan inherits them from its analysis and gets the closing item)
      --topic stringArray              Controlled topic slug (repeatable)
      --type string                    Type: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decision|wiki|lesson|assessment|theme|syllabus|practice|briefing
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
