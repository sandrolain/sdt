## sdt context path

Print the path for a context/ work file

### Synopsis

Print the full path of a context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis/worklog/notes/questions/proposal/prompt/research
(<YYYYMMDD-HHMMSS>-<slug>.md), tasks
(<YYYYMMDD-HHMMSS>-<slug-plan>.md by default, or
<YYYYMMDD-HHMMSS>-<slug-plan>-<stream>.md with --stream <label>;
--phase <n> addresses a section inside the file), tmp (<slug>),
architecture (<slug>.md), wiki (<slug-or/subpath>.md), decision
(<NNNN>-<slug>.md with --number).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type tasks --phase 1 --plan <plan-file>
  sdt context path --type plan --format json
  sdt context path --type decision --number 0001 --slug auth-choice

```
sdt context path [flags]
```

### Options

```
  -h, --help[=true|false]   help for path
      --number string       Number for type decision (4-digit NNNN)
      --phase string        Phase for type tasks (plan phase number, e.g. 1 or 1a)
      --plan string         Plan reference for type tasks (plan file or standalone slug)
      --slug string         Slug (sanitized)
      --stream string       Split-file label for type tasks (kebab-case)
      --type string         Type: plan|analysis|worklog|notes|source|questions|proposal|prompt|research|architecture|decision|tasks|tmp|wiki|lesson|assessment|theme|syllabus|practice|briefing
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
