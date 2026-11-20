## sdt context status vocab

Print the per-kind status vocabulary

### Synopsis

Print the status vocabulary of every document type that carries one, with
the default a new document gets.

WHEN — you are about to set, filter or reason about a document's status and need
the values the validator will accept, rather than a guess or a recollection of
the prose table.

WHEN NOT — you want one document's current status (sdt context status get), or
the per-type document counts (sdt context status). This verb describes what the
vocabulary **is**, not what any document holds.

RETURNS — one block per type: every accepted value in registry order, with the
default marked, plus the type's directory and tier. Both the accepted values and
the default are read from the same registry the validators use, so the answer
cannot drift from what the CLI enforces.

EXAMPLE — sdt context vocab
          sdt context vocab --format json

```
sdt context status vocab [flags]
```

### Options

```
  -h, --help[=true|false]   help for vocab
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

* [sdt context status](sdt_context_status.md)	 - Summarize context/ documents per type with next step
