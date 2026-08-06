## sdt context new

Create a sdt.context/ work file with frontmatter

### Synopsis

Create a plan, worklog or notes file under sdt.context/ with the correct
naming and YAML frontmatter (kind, created_at, context, project). The body comes
from --input/--file or piped stdin. Existing files are preserved unless --force
is set; --edit opens the file in $EDITOR after creation.

Examples:
  sdt context new --type worklog --slug review-deps --input "reviewed deps"
  sdt context new --type plan --slug ship-memory --force

```
sdt context new [flags]
```

### Options

```
      --context string   What triggered this entry
      --edit             Open the file in $EDITOR after creation
      --force            Overwrite existing file
  -h, --help             help for new
      --slug string      Slug (sanitized)
      --type string      Type: plan|worklog|notes
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

* [sdt context](sdt_context.md)	 - Context Tools (sdt.context/ work files)

