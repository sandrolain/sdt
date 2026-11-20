## sdt context set

Write a top-level frontmatter key with byte fidelity

### Synopsis

Write a top-level frontmatter key of a context document. The value is parsed
as a YAML scalar (so 3 is a number, true a bool, [a, b] a list); --raw forces
a string. --append and --remove add or drop one item of a list value. Both YAML
list shapes are read (a block sequence and an inline flow list) and the key's
existing shape is kept: a block sequence stays block, a flow list stays flow,
an absent key becomes a block sequence. A non-empty value that cannot be read as
a list is refused rather than silently shrunk.

Only the target key's bytes are replaced: comments, key order, quoting, block
scalars and the body are preserved. A missing key is inserted after its
declared neighbours.

status is validated against the kind vocabulary and updated follows the
per-type rule; uid, analysis_id and plan_id are refused without --force, since
dedicated commands own them.

Examples:
  sdt context set plan/<plan-file> status active
  sdt context set analysis/x.md categories '[new-feature, research]'
  sdt context set notes/x.md tags cli --append
  sdt context set analysis/x.md summary "a quoted: value" --raw

```
sdt context set <doc> <key> <value> [flags]
```

### Options

```
      --agent string          Provenance: agent/tool that performed the mutation
      --append[=true|false]   Append the value to a list key
      --force[=true|false]    Write a CLI-owned key (uid, analysis_id, plan_id)
  -h, --help[=true|false]     help for set
      --model string          Provenance: model id that performed the mutation
      --number string         Decision number (type decision)
      --phase string          Phase for a task file (type tasks)
      --plan string           Plan reference for a task file (type tasks)
      --raw[=true|false]      Force the value to be written as a string
      --remove[=true|false]   Remove the value from a list key
      --session string        Provenance: session id for traceability
      --slug string           Slug for identity resolution
      --stream string         Split-file label for a task file (type tasks)
      --type string           Document type for identity resolution
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
