## sdt context clean

Empty the discard areas: context/trash and context/tmp

### Synopsis

Empty a discard area. `context rm` moves a document into `context/trash` and
`context/tmp` collects scratch; nothing else removes either, so they grow until
this command is run.

WHEN — the discard areas have served their purpose: the work you might have
wanted back is behind you, and the scratch from a finished task is noise. Run it
alone or per directory as the argument to `sdt agent init --force`, whose obsolete
files land in the same trash.

WHEN NOT — something might still be needed: `--dry-run` lists exactly what would
go and removes nothing, and a single file can be recovered from
`context/trash` until then. There is no rotation, no age threshold and no
recycle bin behind the recycle bin.

RETURNS — per target, the files removed (or that would be) relative to the corpus
root, and their count. A directory that does not exist, or is already empty, is
reported as nothing removed rather than as an error, so a repeated run exits 0.

**A target is required.** An invocation with neither `--trash` nor `--tmp` is
refused before anything is read, and naming the target **is** the confirmation:
there is no second flag to pass, because choosing what to destroy is the whole
decision. Pass both to empty both in one run. Destruction never happens by
omission.

EXAMPLE — `sdt context clean --trash --dry-run`
          `sdt context clean --trash --tmp`

```
sdt context clean [flags]
```

### Examples

```
  sdt context clean --trash --dry-run
  sdt context clean --trash
  sdt context clean --trash --tmp --format json
```

### Options

```
      --apply[=true|false]     With --census: remove the files the census reported
      --census[=true|false]    Report the residue classes (tmp, orphan tasks, stale generated) instead of emptying a target
      --dry-run[=true|false]   List what would be removed without removing it
  -h, --help[=true|false]      help for clean
      --older-than string      Only files last modified before this: a duration (30d, 12h, 3w) or an RFC3339 timestamp
      --tmp[=true|false]       Empty context/tmp
      --trash[=true|false]     Empty context/trash
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
