## sdt context lint

Validate context frontmatter and links

### Synopsis

Validate context frontmatter, Markdown rules and links. With no path,
scans the context corpus; with one or more paths, validates only those documents.
Paths are relative to the project root (context/plan/file.md) or context/
(plan/file.md); targets must be in the same top-level corpus directories scanned
by the default command, or be context/index.md. Wiki pages use
sdt context wiki lint. Exits non-zero when CRITICAL issues are found.

Text output is bounded by default: the first `--limit` issues (20) are printed,
followed by one summary line carrying the full per-severity totals. The totals,
the findings themselves and the exit code are the same whatever the limit —
`--limit 0` prints all of them. --format json and --format yaml always carry every
finding, so a piped query is never shortchanged by the text default.

With --security, additionally scan the selected documents (or the whole corpus)
for prompt-injection phrases, credential/secret literals, invisible/zero-width
Unicode and exfil patterns (advisory WARNING).

With --custody, print the custody report after the findings: four corpus counts
(cited refs/ artifacts without a source sheet, gone sheets whose capture_path
still exists, sheets whose sha256 disagrees with the file they pin, gone sheets
without a restore command) plus what the report cost. The counts are a summary,
never one finding per file, so the lint baseline cannot absorb the class. In
--format json/yaml the counts follow the findings as a separate document.

Examples:
  sdt context lint
  sdt context lint context/plan/example.md
  sdt context lint plan/example.md --format json
  sdt context lint --limit 0
  sdt context lint --security
  sdt context lint --custody
  sdt context lint --fix
  sdt context lint --fix --fix-terminal
  sdt context lint --format json

With --fix, the mechanical findings are repaired before the report: an H1 the
frontmatter title already carries is removed, and a missing `objective`
is filled from the parent document (a plan from its analysis, a task file from
its plan) when the parent answers it. Terminal documents are left untouched
unless --fix-terminal is given, because a rule adopted today does not rewrite a
closed document. Every change is named on stdout.

```
sdt context lint [flags]
```

### Options

```
      --all[=true|false]              Ignore the lint baseline and print every finding
      --baseline-write[=true|false]   Record the current findings as the baseline and report the delta accepted
      --custody[=true|false]          Also print the custody report: four counts over cited refs/ artifacts and source sheets (cited without a sheet, gone with a live capture_path, sha256 disagreement, gone without restore)
      --fix[=true|false]              Repair the mechanical findings (an H1 the title already carries, a derivable objective) before reporting
      --fix-terminal[=true|false]     Let --fix also rewrite completed and archived documents (off by default)
  -h, --help[=true|false]             help for lint
      --limit int                     Maximum issues printed in text output (0: every issue); the totals and the exit code are unaffected (default 20)
      --security[=true|false]         Also scan for prompt-injection, credential and invisible-Unicode patterns (advisory WARNING)
      --stale-days in-progress        Days before an in-progress task file is reported as stale (advisory SUGGESTION) (default 14)
      --suggestions[=true|false]      Also show the suggestions the baseline already accepted
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
