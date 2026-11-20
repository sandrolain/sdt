## sdt grep

Grep File Content

### Synopsis

Search file content with a regular expression over the working tree.

WHEN — you need the lines of the tree that match a pattern and have no host
search tool. The context/ corpus is searched like any other directory, so you can
ask what the project's own records say about a subject.

WHEN NOT — you need a document from the corpus by what it means rather than by
the exact string it contains: `sdt context search` ranks documents and filters by
kind, status and date. You want the captured third-party clones under
context/refs (pass `--refs`); they are pruned by default because they are 9,2 GB
of third-party code. Or you want to prune a large subtree: `--exclude` filters
the **results**, so the walk still descends into it — use `--dir` to point at
the part you want instead. `--dir` names a directory: a file path is refused
with the reason, never answered with an empty result; to search one file, pass
`--path <file>`.

RETURNS — one entry per matching line (path, 1-based line number, text) plus the
counts of files searched and matched, and `truncated` when a cap stopped the scan
early. Output is bounded by default: 200 matches overall, unlimited per file
unless `--max-count` is set.

`-A n`, `-B n` and `-C n` (GNU-compatible) add the n lines around each match,
so a match is read in its surrounding without a second command. Context lines are
**not** matches: they never satisfy the pattern, never count toward either cap,
and never extend the scan — a match past the cap brings no context with it. In
text they render as `path-line-text` against a match's `path:line:text`, with `--`
between non-adjacent blocks; a line shared by two matches is printed once and
keeps its match marker. Both sides of an overlap are merged, not concatenated.
`-C n` is exactly `-A n -B n`; an explicit side wins over `-C` for that side.

`-l` (`--files-with-matches`) answers "which files" instead of "where": one path
per file carrying a match, sorted, in `paths` with `matches` left empty. It reads
each file only to its first match and `--limit` bounds the list of **paths**. The
flags that describe how to render a match — `--max-count`, `-A`, `-B`, `-C` — are
**refused** in this mode rather than ignored, because a caller who sets one believes
it took effect.

`-c` (`--count`) reports how many lines match instead of which ones — the
`grep -c` fallback. The count is a fact rather than a rendering, so
`--limit` is refused with it; `--max-count` is honoured and marked `truncated` when
it clipped, which is what `grep -m N -c` counts. Counting every line of a file is
`sdt read --count`, not a grep with an empty pattern.

The scope, stated plainly: no ignore file is consulted, so a path listed in
.gitignore is searched like any other. The directories pruned are the ones your
project declares as `skipDirs` in .sdt.yaml when it declares any, and the shipped
defaults (.git, .hg, .svn, node_modules, .venv) when it does not. A declaration
replaces the tool residues, not the VCS ones: .git, .hg and .svn are pruned
whatever you declare. context/refs and context/tmp are pruned unless `--refs` is
given. Binary files are not read.

EXAMPLE — `sdt grep "replaceAll" --include "**/*.ts" --limit 40`

Both caps are presentation, never correctness: `--limit 0` reports every match.
A path pattern is matched against the whole path, so `context/analysis/*.md` does
not reach `src/legacy/context/analysis/*.md`; `**/` is how you ask for any depth.

```
sdt grep <pattern> [flags]
```

### Examples

```
  sdt grep "TODO" --format json
  sdt grep "TODO" --include "**/*.go" --exclude "**/*_test.go"
  sdt grep "cascade rule" --include "context/**/*.md"
  sdt grep "handleRequest" --include "**/*.go" -A 12
  sdt grep "deprecated" --dir src -l --limit 0
  sdt grep "handleRequest" --include "**/*.go" -c
  sdt grep "benchmark" --refs --limit 0
```

### Options

```
  -A, --after int                         Print the N lines after each match
  -B, --before int                        Print the N lines before each match
  -C, --context int                       Print the N lines before and after each match
  -c, --count[=true|false]                Print the number of matching lines instead of them
      --dir string                        Directory to search; a file path is refused (default: the working directory)
      --exclude string                    Skip paths matching this glob
  -l, --files-with-matches[=true|false]   Print only the paths of files carrying a match
  -h, --help[=true|false]                 help for grep
  -i, --ignore-case[=true|false]          Case-insensitive match
      --include string                    Only search paths matching this glob
      --limit int                         Maximum matches reported overall (0: the default cap)
      --max-count int                     Maximum matches reported per file (0: unlimited)
      --no-ignore[=true|false]            No longer needed: the raw tree is already the default (accepted for compatibility)
      --path string                       Search exactly one file (a directory is refused)
      --refs[=true|false]                 Also search the captured clones under context/refs (pruned by default)
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
