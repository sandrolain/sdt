## sdt context rm

Discard a document by moving it into context/trash

### Synopsis

Discard a document: move the file into `context/trash`, leaving its bytes
untouched.

WHEN — the file itself must go: a wrong capture, a scratch document, something the
session created and should not have. This is an **editing** command, not a
management one; `sdt context deprecate` is the command for "keep it, stop
trusting it".

WHEN NOT — the document is still worth reading: `deprecate` retires it in place
and no reference breaks. You want the corpus left consistent and are willing to
spend the round trips: this command moves the file and **reports** the documents
that referenced it rather than repairing them.

RETURNS — the path, the type, where the file went, and the documents that named
it, so the caller can decide what to do about them. The report is the point: a
reference into the trash would survive until the first `clean --trash` and then
break with nothing to notice it, so the repair is left where the decision is.

The move is recoverable by construction — the file is one directory away — which
is why there is no confirmation flag to pass. A name already present in the trash
is not overwritten: the file lands with a timestamp instead.

EXAMPLE — `sdt context rm notes/20260101-000000-scratch`
          `sdt context rm --type notes --slug scratch --dry-run`

```
sdt context rm <ref> [flags]
```

### Examples

```
  sdt context rm notes/20260101-000000-scratch
  sdt context rm analysis/20260101-000000-abandoned --dry-run
  sdt context rm --type notes --slug scratch --format json
```

### Options

```
      --dry-run[=true|false]   Report the move and the referrers without moving anything
  -h, --help[=true|false]      help for rm
      --number string          Decision number (type decision)
      --phase string           Phase for a task file (type tasks)
      --plan string            Plan reference for a task file (type tasks)
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
