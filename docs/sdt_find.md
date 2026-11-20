## sdt find

Find Files By Path

### Synopsis

List the working-tree paths matching a glob.

WHEN — you need the paths of files you will read, and have no host file-listing
tool. The context/ corpus is listed like any other directory.

WHEN NOT — you need the corpus work files with their status and date:
`sdt context list --type <k>`. You want the captured third-party clones under
context/refs (pass `--refs`), which are pruned by default for size. Or you are
about to hand this a glob to prune a large subtree — that does not work here,
because the descent happens before the glob is applied; use `--dir` to point at
the part you want. `--dir` names a directory: a file path is refused with the
reason rather than listed as an empty result.

RETURNS — the matching paths, sorted, relative to the search root, each marked
when it is a directory. Omitting the glob lists every file in scope. A pattern
without a slash matches a basename at any depth (`*.md` finds every markdown
file); a pattern with a slash is matched against the **whole** path, so
`context/analysis/*.md` reaches only that directory — prefix it with `**/`
to ask for any depth.

A printed path round-trips: feed it back to reach exactly that file.

The scope, stated plainly: no ignore file is consulted, so a path listed in
.gitignore is listed like any other. The directories pruned are the ones your
project declares as `skipDirs` in .sdt.yaml when it declares any, and the shipped
defaults (.git, .hg, .svn, node_modules, .venv) when it does not. A declaration
replaces the tool residues, not the VCS ones: .git, .hg and .svn are pruned
whatever you declare. Patterns are relative to the directory holding .sdt.yaml
and keep that meaning under `--dir`.
context/refs and context/tmp are pruned unless `--refs` is given.

EXAMPLE — `sdt find "**/*_test.go" --format json`
          `sdt find "context/decisions/*.md" --limit 5`
          `sdt find "context/refs/**/README.md" --refs --limit 3`
          `sdt find context/instructions/cli.md`

```
sdt find [glob] [flags]
```

### Examples

```
  sdt find "*.tmpl"
  sdt find "context/analysis/*.md" --format json
  sdt find --dir context/notes --limit 0
```

### Options

```
      --dir string               Directory to list; a file path is refused (default: the working directory)
      --dirs[=true|false]        Include directories in the result
  -h, --help[=true|false]        help for find
      --limit int                Maximum paths reported (0: unlimited)
      --no-ignore[=true|false]   No longer needed: the raw tree is already the default (accepted for compatibility)
      --refs[=true|false]        Also list the captured clones under context/refs (pruned by default)
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
