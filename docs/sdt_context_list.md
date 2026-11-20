## sdt context list

List context/ work files

### Synopsis

List existing work files under context/ for a type, sorted by name
(chronological for timestamped files).

Types: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decisions|tasks|wiki|lesson|assessment|theme|syllabus|practice|briefing.

Examples:
  sdt context list --type worklog
  sdt context list --type plan --status active --last 7d
  sdt context list --type analysis --where categories=refactor

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
sdt context list [flags]
```

### Options

```
      --after string          Match documents dated on/after (YYYY-MM-DD, RFC3339, now, now-<N><unit>)
      --agent agent           Filter by frontmatter agent provenance
      --before string         Match documents dated on/before (same forms as --after)
      --category categories   Filter by frontmatter categories (repeatable; any-match)
      --date string           Reference timestamp: created (default) or updated
  -h, --help[=true|false]     help for list
      --last string           Match documents in the last window, e.g. 6h, 7d, 3w (exclusive with --after)
      --role role             Filter by frontmatter role provenance
      --since string          Alias of --after
      --status string         Filter by frontmatter status
      --type string           Type: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decisions|tasks|wiki|lesson|assessment|theme|syllabus|practice|briefing
      --until string          Alias of --before
      --where stringArray     Generic frontmatter filter key=value or key!=value (repeatable; AND-ed)
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
