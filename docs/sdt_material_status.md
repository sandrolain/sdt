## sdt material status

Report freshness, review and publishability

### Synopsis

Report the three states of each guide, one row per concept and language.

Freshness is the render lane's verdict (fresh/stale/missing/orphan). Review comes
from the ledger beside the guide: materialReviewNotRun when the critic or the reader is
empty or says so, "pending" when one says "to run", materialReviewRecorded otherwise — the
raw text is carried, never rewritten into "passed". Publishable requires all
three: fresh, a recorded review, and a classification that is not "undeclared".
Exit 0: a status is a report, not a gate.

```
sdt material status [<concept>] [flags]
```

### Options

```
  -h, --help[=true|false]   help for status
      --lang string         language to filter, e.g. it
      --syllabus string     syllabus to filter, with or without syllabi/
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

* [sdt material](sdt_material.md)	 - Export a study guide without its authoring machinery
