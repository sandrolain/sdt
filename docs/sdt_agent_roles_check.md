## sdt agent roles check

Validate the role profiles deterministically

### Synopsis

Validate the generated role profiles under
`context/roles/`, no runtime required:

  - closed register + profile presence: the on-disk set must equal the register
    exactly (shared.md + one `<slug>.md` per register slug);
  - drift guard: every profile's generated `core`/`project`
    layers must match a fresh render (user-owned `preferences` is never
    treated as drift);
  - owner/reviewer overlap: no path may be claimed by more than one role.

Exits non-zero when any CRITICAL finding exists. Advisory mirrors of these
checks surface in `sdt context lint` and `sdt agent doctor`.

Examples:
  sdt agent roles check
  sdt agent roles check --format json

```
sdt agent roles check [flags]
```

### Options

```
  -h, --help[=true|false]   help for check
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

* [sdt agent roles](sdt_agent_roles.md)	 - Role register and generated role profiles
