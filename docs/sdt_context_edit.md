## sdt context edit

Replace a document body, or one of its sections, from input

### Synopsis

Replace the body of a context document, or one of its sections, with text read
from --input, --file, --inb64 or piped stdin.

The frontmatter is never written. A frontmatter key goes through
sdt context set; an input that carries a leading frontmatter block is refused
rather than silently dropped, so an edit can never look like it changed a key
when it did not.

<doc> is resolved in this order: a path under context/ (absolute, context/…, or
kind/name); a bare file name that matches exactly one document; a wiki page id
(for example architecture/deployment-pipeline) or a wiki page file name at any
depth; a document uid. The first form that matches wins, and several matches are
refused with the candidates rather than guessed.

WHEN — the body of an existing document must change and the new text is in hand
(a heredoc, a file, a pipe, the output of an editor).

WHEN NOT — a frontmatter key changes: use sdt context set <doc> <key> <value>.
A whole new document is created: use sdt context new.

RETURNS — the unified diff between the document as it is and as it would be,
then the write. --dry-run prints the diff and writes nothing. When the new text
leaves the body byte-identical the command reports "no change" and touches
nothing; the CLI-owned updated field is refreshed on a real edit, as with
sdt context set.

A SECTION IS THE HEADING AND ITS BODY — --section replaces the whole section, so
the text must begin with that section's heading, at its own level (for example
## Claims). A body without the heading is refused rather than spliced in, which
would delete the anchor a reader navigates by. A subsection is a section of its
own: --section Review leaves ### Gate untouched, and a body that repeats a
surviving heading is refused instead of duplicating it.

--append ADDS INSTEAD OF REPLACING — the text goes after the body's last line,
or after the last line of --section, and carries no heading. It is the shape for
one checklist item, one section, one log line: the worklog and note bodies that
used to be re-sent whole through --file.

AN EMPTY BODY IS A DESTRUCTION — it empties the document, or removes the section
when --section is named. A body source that yields no text is refused unless
--force is given, and the refusal names the document; a caller that pipes the
wrong file cannot wipe a page by accident. --force covers emptiness only: it
never accepts a frontmatter block or a section body without its heading.

EXAMPLE — sdt context edit plan/x --section wave-2 --file wave-2.md
          printf '## Findings\n…\n' | sdt context edit research/x --dry-run

```
sdt context edit <doc> [flags]
```

### Examples

```
  sdt context edit plan/x --section wave-2 --file wave-2.md
  printf '## Findings\n' | sdt context edit research/x --dry-run
```

### Options

```
      --agent string           Provenance: agent/tool that performed the mutation
      --append[=true|false]    Append the input after the body, or after --section, instead of replacing
      --dry-run[=true|false]   Print the diff without writing
      --fence string           Replace the fenced block by 1-based index or by its opening info string
      --force[=true|false]     Allow an empty body (empties the document, or removes the section)
  -h, --help[=true|false]      help for edit
      --model string           Provenance: model id that performed the mutation
      --number string          Decision number (type decision)
      --phase string           Phase for a task file (type tasks)
      --plan string            Plan reference for a task file (type tasks)
      --row string             Replace the table row whose first cell is this literal
      --section string         Edit only this section (its stable id or its heading)
      --session string         Provenance: session id for traceability
      --slug string            Slug for identity resolution
      --stream string          Split-file label for a task file (type tasks)
      --type string            Document type for identity resolution
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
