## sdt learn course

Read a theme or a syllabus: membership, readiness and order conflicts (read-only)

### Synopsis

Read one course level: a theme (an id starting with `themes/`) lists lessons, a
syllabus (an id starting with `syllabi/`) lists themes. The verb resolves every
member in file order — position is order — and reports what a reader resolves.

WHEN — before teaching a course, after authoring a level, or when a membership
must be checked before it is published.
WHEN NOT — you want one concept's lesson: `sdt learn lesson`. You want to know
whether a concept is teachable: `sdt learn check`.

RETURNS — the level, its members in order, each lesson's readiness verdict, the
refusals and the order conflicts. A dangling, archived, duplicate or wrong-kind
member is a refusal and exits 1. An order that contradicts a member's
`relations.depends_on` or `## Prerequisites` is reported and never resolved.
`--validate` reads every level and counts both as findings. Reads only.

EXAMPLE — sdt learn course themes/delivery-metrics
          sdt learn course syllabi/software-architect --format json
          sdt learn course --validate

```
sdt learn course <id> [flags]
```

### Options

```
      --coverage[=true|false]   Print the objective to item matrix across the membership (read-only)
  -h, --help[=true|false]       help for course
      --validate[=true|false]   Read every theme and syllabus and report findings (read-only)
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
