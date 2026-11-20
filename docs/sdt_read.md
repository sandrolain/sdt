## sdt read

Read a file or a line range of it

### Synopsis

Print a file, optionally only a line range.

WHEN — you need the content of a file, or a slice of it, and have no host
reader.

WHEN NOT — you need a document from the corpus by what it is: `sdt context show <path>`
prints a `context/` work file, and `--section` selects one of its sections without
counting lines.

RETURNS — the file content, or lines `--lines <from>:<to>` of it, 1-based and inclusive. A
range that starts past the last line, or that is inverted, is an error rather
than an empty answer; an upper bound past the last line clamps to the end.

`--count` reports the number of lines instead of them — the `wc -l` fallback. It
composes with `--lines`, counting the selection rather than contradicting it, and a
file with no lines has a count of 0.

`--numbered` prefixes each printed line with its 1-based number in the file, the
shape a host reader prints, so a citation can be quoted verbatim. `--lines -N:`
selects the last N lines — `tail -n` without a second verb.

Earlier versions ignored `--format json` here and printed raw text: a global flag
that silently did nothing is the defect the change fixed. The JSON answer is an
object `{path, lines, count, numbered}`.

EXAMPLE — `sdt read -p cli/cmd/file.go --lines 31:49`


```
sdt read [flags]
```

### Examples

```
  sdt read -p go.mod
  sdt read -p cli/cmd/context_search.go --lines 168:214
  sdt read -p go.mod --count
```

### Options

```
      --count[=true|false]      Print the number of lines instead of them (with --lines: of the range)
  -p, --file string             File path
  -h, --help[=true|false]       help for read
      --lines string            Print only this line range (from:to, 1-based)
  -n, --numbered[=true|false]   Prefix each line with its 1-based number
      --section string          Print only this section (by id or heading)
      --sections[=true|false]   List the document's sections (id, level, heading)
```

### Options inherited from parent commands

```
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
