## sdt edit

Replace one literal occurrence in a file, guarded and atomic

### Synopsis

Replace one literal occurrence in a source file.

WHEN — a file needs a surgical change and the exact text is in hand: renaming a
call, fixing a constant, changing one branch. It is the verb a `python3` heredoc or
`sed -i` was standing in for.

WHEN NOT — a corpus document changes: `sdt context edit` owns `context/` (body,
section, append) and this verb refuses it. A whole file is being written:
`sdt write`. Several hunks in one operation: a patch is the host's job
(`git apply`).

RETURNS — the unified diff between the file as it is and as it would be, then
the write. `--dry-run` prints the diff and writes nothing; an `--old` equal to
`--new` reports "no change" and touches nothing.

`--batch` edits several files from one JSON or YAML manifest, so the
N-files × M-pairs fallback (`python3 - <<PY` with per-pair assertions) is one call:
each entry names a file and its literal pairs, each pair may pin the occurrence
count it expects (`count: 0` is a guard that refuses when the literal occurs),
every pair asserts before anything is written, and each file is one atomic
transaction — a refused file is left byte-identical while the others apply. The
exit is 0 when every file applied, 3 when some applied and some were refused,
1 when nothing applied. Read the manifest from a path, or pass `-` (or nothing)
after `--batch` to read it from stdin. Pairs apply in order: a later pair sees
the content its predecessors replaced.

THE GUARD IS THE POINT — a literal must occur exactly once. Zero occurrences is
an error that names the file; more than one is an error that names the count and
points at `--all`. Guessing which occurrence was meant is how a literal edit
corrupts a source file, so the verb refuses instead.

IT REFUSES WHAT IT MUST NOT TOUCH — a path under `context/`, a symlink, a binary
file, a file that is not valid UTF-8, and a path outside the project root when
the project declares one. The write is atomic: a temp file in the same
directory, the original mode, then a rename, so a failed write leaves the file
as it was.

A CRLF FILE STILL MATCHES — the `
` in `--old` and `--new` is converted to the file's
own newline, so a shell literal does not silently fail to match.

EXAMPLE — `sdt edit -p cli/cmd/file.go --old 'not exist' --new 'not found'`
          `sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run`
          `sdt edit --batch batch.yaml`

```
sdt edit [flags]
```

### Examples

```
  sdt edit -p cli/cmd/file.go --old 'old text' --new 'new text'
  sdt edit -p AGENTS.md --old 'x' --new 'y' --all
  sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run
```

### Options

```
      --all[=true|false]       Replace every occurrence instead of requiring exactly one
      --batch string           Edit N files from a JSON/YAML manifest of file entries with literal pairs (path, or "-"/empty reads stdin)
      --dry-run[=true|false]   Print the diff without writing
      --expect string          Refuse unless the block's sha256 starts with this hex prefix (at least 8)
  -p, --file string            File to edit
      --from-marker string     Literal that opens the block to replace (with --to-marker)
  -h, --help[=true|false]      help for edit
      --lines string           Replace the line range a:b (1-based, inclusive)
      --new stringArray        Replacement text (empty deletes; repeat for a batch)
      --new-file string        Read the replacement text from this file
      --old stringArray        Literal text to replace (repeat for a batch; must occur exactly once)
      --old-file string        Read the literal to replace from this file
      --regex[=true|false]     Treat --old as an RE2 pattern and expand $1 in --new
      --to-marker string       Literal that closes the block to replace (with --from-marker)
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
