## sdt learn resume

Resume the learning session: profile, last session and the review queue (read-only)

### Synopsis

Report the learner's profile, the last recorded session and what is due.

WHEN — a session resumes and the agent needs the standing before teaching.
WHEN NOT — you want the full picture (goals, coverage, weak areas): use
sdt learn status.

RETURNS — learner, last session, goal count, the review queue split into due and
upcoming, and at most two ranked next actions, each with the reason it was chosen
(due > weak > blocked prerequisite > new). An absent learner state reports an empty
session rather than failing. Reads only.

EXAMPLE — sdt learn resume
          sdt learn resume --format json

```
sdt learn resume [flags]
```

### Options

```
  -h, --help[=true|false]   help for resume
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

* [sdt learn](sdt_learn.md)	 - Learn, review and record concepts against the local wiki
