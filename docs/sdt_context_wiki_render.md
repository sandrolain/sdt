## sdt context wiki render

Report the freshness of the derived lane, or gate on it

### Synopsis

Compare the derived lane with what it derives from: every technical render
(context/renders/technical/<page-id>.md) with the wiki page it was generated from,
and every material (context/renders/materials/<syllabus>/<concept>.<lang>.md) with
the claims its header pins.

STATUS — fresh (the stamp equals the page's updated), stale (it differs, or the
header cannot be read), missing (a page named on the command line has no render),
orphan (a render whose page does not exist). The walk finds the renders; a page
owes a render only when this command is asked about it.

A MATERIAL IS THE SECOND SHAPE OF THE SAME LANE (decision/0038): no kind, no
frontmatter, its contract on the first line — concept, syllabus, language,
classification and one claim id to digest pair per asserted claim. Its statuses
reuse the four words: fresh (every digest still matches), stale (one moved, or the
header cannot be read), missing (a claim the page no longer carries), orphan (no
page for the concept). It reports under `lane: material` in JSON.

THE PROSE IS CHECKED TOO (decision/0040): every claim marker must resolve to a live
claim at the digest it pins, every lesson marker to a section the lesson carries,
and every gloss marker to a non-empty term, and every objective marker to one of the
page's learning objectives, each number once (decision/0041); a guide whose prose
marks nothing is stale, because silence is not evidence. The check line that closes
an exercise (decisions 0046, 0047) carries an assess marker naming the concept's
item bank: every id it lists exists, and no item of the bank is left out. The ids sit
in the marker, not in the reader's sentence, which says only that a check exists. A
deck carries that one marker too, because a reader of the slides alone needs the
same pointer. A deck (<concept>.<lang>.slide.md)
is otherwise exempt:
its content is an outline and its speaker notes are prose, not assertions. The first
failure is reported, not a count.

EXIT — 0 by default: the report is a report. With `--check` it exits 1 when
anything in either lane is not fresh, so a caller that quotes a render or a material
gates on it first.

EXAMPLE — sdt context wiki render
          sdt context wiki render --check
          sdt context wiki render backend/auth --format json

```
sdt context wiki render [wiki-id...] [flags]
```

### Options

```
      --check[=true|false]   Exit 1 when anything in either derived lane is not fresh (the gate); without it the report always exits 0
  -h, --help[=true|false]    help for render
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
