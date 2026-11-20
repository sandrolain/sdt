## sdt agent roles init

Generate role profiles under context/roles/

### Synopsis

Generate the base-first three-layer role profiles under
`context/roles/<slug>.md` plus the shared rules file
`context/roles/shared.md`.

Non-destructive by default: existing profiles are preserved. Use --force to
refresh the generated core and project layers in place — the user-owned
preferences scope is never overwritten. The project layer is derived from repo
evidence (stack, layout, real build/test/lint commands); use --ask to answer
the ≤3 non-derivable role facts interactively. Project identity comes from
--project, else .sdt.yaml.

Examples:
  sdt agent roles init
  sdt agent roles init --force
  sdt agent roles init --ask
  sdt agent roles init --project myapp --force

```
sdt agent roles init [flags]
```

### Options

```
      --ask[=true|false]     Ask up to 3 non-derivable role facts (interactive)
      --force[=true|false]   Refresh generated core/project layers (user preferences preserved)
  -h, --help[=true|false]    help for init
      --project string       Project id (default: from .sdt.yaml)
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
