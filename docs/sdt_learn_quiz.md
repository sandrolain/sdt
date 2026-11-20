## sdt learn quiz

Generate (or validate) a zero-hint quiz manifest for selected concepts

### Synopsis

Build a quiz manifest from the selected concepts, or validate one.

WHEN — a quiz is about to be run, or a manifest must be checked before use.
WHEN NOT — you want the concepts themselves: use sdt learn inspect. A concept with
a lesson carries that lesson's questions; one without a lesson carries evidence only
and says so, and its questions are yours to write.

RETURNS — a manifest (version, hinted and keyed flags, one entry per concept with
its title, evidence and — when the concept has a lesson — its items as
prompt/type/claim) or, with `--validate <file>`, the validation verdict. It
carries no expected answer: zero-hint by default, and `--hint` opts into the
page summary only. `--with-key` adds the frozen key, its grading rule and its
provenance, which is what `sdt learn answer` grades against and the only way the
key enters a manifest. An unknown concept fails rather than yielding a guess. Reads
only.

EXAMPLE — sdt learn quiz backend/auth --format yaml
          sdt learn quiz backend/auth --with-key --format json
          sdt learn quiz --validate quiz.yaml

```
sdt learn quiz <concept>... [flags]
```

### Options

```
  -h, --help[=true|false]       help for quiz
      --hint[=true|false]       Include the page summary as a hint (zero-hint by default)
      --validate string         Validate an existing manifest file instead of generating one
      --with-key[=true|false]   Include each item's frozen key, grading rule and provenance
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
