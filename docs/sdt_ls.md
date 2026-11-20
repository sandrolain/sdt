## sdt ls

List a directory, one or more levels deep

### Synopsis

List the entries of a directory.

WHEN — you need to know what is in a directory, and how large the entries are:
the first step of nearly every session on an unfamiliar tree.

WHEN NOT — you know the shape of the path you want: `sdt find <glob>` walks by
pattern and prints paths only. You want one file's contents: `sdt read -p`.

RETURNS — one line per entry, directories first then by path, as `d` or `f`
then the path. `--stat` adds a size and a modification time; `--depth` descends
that many levels (1 by default, the directory itself); `--limit` caps the rows
and marks the answer truncated.

The walk prunes what `find` prunes: the project's declared `skipDirs` and the
shipped defaults, plus `context/refs` and `context/tmp` — a listing that walked
9,2 GB of captured clones would not be a listing.

EXAMPLE — `sdt ls --stat --depth 2`
          `sdt ls --dir cli/cmd --limit 40`

```
sdt ls [flags]
```

### Examples

```
  sdt ls
  sdt ls --stat
  sdt ls --dir context/analysis --limit 20
```

### Options

```
      --depth int           How many levels below --dir to list (default 1)
      --dir string          Directory to list (default: the working directory)
  -h, --help[=true|false]   help for ls
      --limit int           Maximum entries reported (0: unlimited)
      --stat[=true|false]   Add size and modification time
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
