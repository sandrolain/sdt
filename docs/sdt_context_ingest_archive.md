## sdt context ingest archive

Move an ingestion source to context/refs/ after a hash check

### Synopsis

Move one source from context/ingestion/ to context/refs/<basename>.

The destination must not exist. The source's sha256 is checked before the move;
an unconverted source is refused unless --force is set, because archiving is the
step after distillation. A sidecar <basename>.source.yaml records the hash and
the converted artifact, so the archive is auditable without frontmatter on the
binary. A source whose name an agent runtime resolves as instructions (AGENTS.md,
CLAUDE.md, GEMINI.md, SKILL.md and the other names in the closed list) is stored
as <basename>.source and the sidecar records original_name: a third party's
instructions must not arrive loadable. --dry-run prints the plan and writes
nothing.

```
sdt context ingest archive <source> [flags]
```

### Options

```
      --classification string   What the source declares about its use: public|internal|confidential
      --dry-run[=true|false]    Print the archive plan without moving the file
      --force[=true|false]      Archive even when no converted artifact matches
  -h, --help[=true|false]       help for archive
      --license string          The source's licence (recorded as declared, never interpreted)
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

* [sdt context ingest](sdt_context_ingest.md)	 - Track sources in context/ingestion/ (list, status, archive, plan)
