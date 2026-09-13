## sdt context new

Create a context/ work file with frontmatter

### Synopsis

Create a plan, analysis, worklog, notes, questions, RFC, prompt, architecture or ADR
file under context/ with the correct naming and the full per-type YAML
frontmatter (kind, summary, context, status, created, updated, project plus
per-type fields). The body comes from --input/--file or piped stdin. Existing
files are preserved unless --force is set; --edit opens the file in $EDITOR
after creation.

The slug is derived from --title when --slug is omitted; --summary is optional
and falls back to a MANDATORY-fill placeholder so the file passes lint. For
ADR type the next NNNN number is auto-assigned (override with --number). The
command prints the created file path (--format text|json|yaml).

Examples:
  sdt context new --type worklog --title "review deps" --input "reviewed deps"
  sdt context new --type plan --title "ship memory" --force
  sdt context new --type analysis --title "memory backend" --input "..."
  sdt context new --type architecture --title "config loading" --summary "config loading component"
  sdt context new --type adr --title "Auth choice" --summary "Use JWT for auth"
	sdt context new --type questions --title "open api questions"
	sdt context new --type rfc --title "add prompt provenance"
	sdt context new --type prompt --title "deepsearch prompt"

```
sdt context new [flags]
```

### Options

```
      --context string   What triggered this entry
      --edit             Open the file in $EDITOR after creation
      --force            Overwrite existing file
  -h, --help             help for new
      --number string    Override for the ADR number (default: next NNNN from decisions/)
      --slug string      Slug (sanitized; overrides --title-derived slug)
      --summary string   Summary for the frontmatter (default: MANDATORY-fill placeholder)
      --title string     Title (slug derived from it when --slug omitted)
      --type string      Type: plan|analysis|worklog|notes|questions|architecture|adr
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)

