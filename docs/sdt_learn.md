## sdt learn

Learn, review and record concepts against the local wiki

### Synopsis

Resolve a topic to the wiki concepts that cover it, report what is due or
weak, and record what the learner demonstrated.

WHEN — you need to learn, review, quiz or practise a concept, or to explain one
from the corpus.
WHEN NOT — the corpus holds no page for the topic: this surface reports what the
corpus has, it never teaches from recall. Ingesting a source is a different job.

RETURNS — reports over the wiki graph and the learner state. Only `learn record`
writes, and only on an explicit call.

EXAMPLE — sdt learn inspect "resolution order"
          sdt learn review --limit 5

### Options

```
  -h, --help[=true|false]   help for learn
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

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt learn answer](sdt_learn_answer.md)	 - Grade one answer against an item's frozen key (read-only)
* [sdt learn check](sdt_learn_check.md)	 - Report whether a concept can be taught, and why not (read-only)
* [sdt learn course](sdt_learn_course.md)	 - Read a theme or a syllabus: membership, readiness and order conflicts (read-only)
* [sdt learn eval](sdt_learn_eval.md)	 - Run the behaviour cases under context/learning/evals (read-only)
* [sdt learn inspect](sdt_learn_inspect.md)	 - Resolve a topic to wiki concepts and their prerequisites (read-only)
* [sdt learn lesson](sdt_learn_lesson.md)	 - Compose a lesson from its page and its lesson document (read-only)
* [sdt learn quiz](sdt_learn_quiz.md)	 - Generate (or validate) a zero-hint quiz manifest for selected concepts
* [sdt learn record](sdt_learn_record.md)	 - Record an assessment, a goal or a session in the learner state (the only writer)
* [sdt learn resume](sdt_learn_resume.md)	 - Resume the learning session: profile, last session and the review queue (read-only)
* [sdt learn review](sdt_learn_review.md)	 - List the due and weak concepts as a bounded review queue (read-only)
* [sdt learn status](sdt_learn_status.md)	 - Report goals, coverage, weak areas and due reviews (read-only)
