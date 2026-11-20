## sdt context wiki claims

Flag broad claims that cite too many lines or units

### Synopsis

Flag claims whose citation is too broad to be atomic.

A claim block starts at its {#claim-N} anchor and runs to the next anchor. For
each citation in the block the command counts the cited artifact lines and the
artifact units the span overlaps. A citation over --max-lines (default 40) or
over --max-units (default 2) is a SUGGESTION: the command names the page, claim
id, citation, line count and unit count, exits 0 and never edits a page.

With --validate the same walk selects the validation shortlist instead: one row
per target, where a target is a page whose claims all rest on one source (or
whose maturity or validity says they age) or a single claim whose own prose
carries "inferred" or "not stated in the source". The report only selects; a
person decides (ingestion.md, *The validation lane*).

Examples:
  sdt context wiki claims
  sdt context wiki claims architecture/deployment-pipeline --max-lines 80
  sdt context wiki claims --format json
  sdt context wiki claims --validate
  sdt context wiki claims --validate --horizon 30 --format json

```
sdt context wiki claims [page...] [flags]
```

### Options

```
      --at int                       With --budget, lower the threshold (default: the lint's constant)
      --budget[=true|false]          Report pages over the claim budget, worst first
      --corroboration[=true|false]   Report claim pages linked to the same concept as corroboration candidates
  -h, --help[=true|false]            help for claims
      --horizon int                  With --validate, days ahead that make an evolving claim due (default 180)
      --max-lines int                Flag a citation longer than this many artifact lines (default 40)
      --max-units int                Flag a citation overlapping more than this many artifact units (default 2)
      --validate[=true|false]        Report the validation shortlist: pages and claims that deserve a second source
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

* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)
