## sdt learn record

Record an assessment, a goal or a session in the learner state (the only writer)

### Synopsis

Append or update the learner state — on an explicit call, never as a side effect.

WHEN — the learner demonstrated, applied or recalled a concept; a goal was declared;
a session ended.
WHEN NOT — you are about to check an answer: the check is read-only, and only what
you decide afterwards is recorded here.

RETURNS — what was written to context/learning/learner.yaml. Provenance is explicit:
`--basis` defaults to inferred (the weaker claim). An unknown concept fails.

A graded call (`--grade --item`) is the only way an assessment is written,
because it is the only one that carries the attempt and the date: the grade sets the
standing (`incorrect` → weak, `partially_correct` → partial,
`correct` → partial, or solid only after an earlier demonstration), `--hint`
caps it at partial when above 0, and the review date is computed with its reason
(`failure` 1d, `correct_but_hesitant` 3d, `independent_success` 7d,
`transfer_success` 14d, `durable_success` 30d) unless `--next-review`
names one. `ungradable` writes nothing and exits 4, and so does a bank whose
`key_hash` or `hash` no longer agrees with its row: one condition, one code. A call with no grade
records exposure — the record, its evidence, its misconception — and leaves the
standing untouched.

EXAMPLE — sdt learn record --concept architecture/four-key-metrics --item fkm-3 --grade correct --hint 0
          sdt learn record --concept backend/auth --evidence refs/auth.md@abcd1234:9-10 --basis observed
          sdt learn record --profile "a learner" --since 2026-10-13
          sdt learn record --concept backend/auth --session --covered backend-auth

```
sdt learn record [flags]
```

### Options

```
      --basis string                Provenance of the rating: observed or inferred (default inferred)
      --concept string              Wiki page id of the concept being assessed
      --confused-with stringArray   Wiki id of the concept it was mistaken for (repeatable)
      --covered stringArray         Concept record id covered in the session (repeatable)
      --due stringArray             Concept record id that became due (repeatable)
      --evidence stringArray        Evidence locator backing the record (repeatable)
      --goal stringArray            Goal as "<wiki-id>=<outcome>" (repeatable)
      --goal-audience string        Who the goal's outcome is declared for
      --goal-status string          Goal lifecycle: open, working, closed (closed stamps the date)
      --grade string                Graded answer: correct, partially_correct, incorrect, ungradable (the standing is derived from it)
  -h, --help[=true|false]           help for record
      --hint int                    Hint rung the answer needed, 0 for none (caps the standing)
      --item string                 Item id the answer was given on (required with --grade)
      --misconception string        Open misconception to record, as the learner stated it
      --misconception-kind string   Typed half of the misconception: overgeneralized, conflated_terms, wrong_precondition, missing_distinction, false_causality, stale_model
      --next-review string          Next review date (YYYY-MM-DD)
      --prerequisite stringArray    Prerequisite record id (repeatable; defaults to the page's relations)
      --profile string              Learner name to record
      --session[=true|false]        Append a session-log entry
      --since string                Learning start date (YYYY-MM-DD)
      --weak stringArray            Concept record id that stayed weak (repeatable)
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
