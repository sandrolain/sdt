## sdt agent roles show

List the closed role register

### Synopsis

Render the closed role register: every canonical role slug, its title,
the coordinator flag and the default owned path globs. The register is the
single source of truth for the `role:` frontmatter vocabulary.


```
sdt agent roles show [flags]
```

### Options

```
  -h, --help[=true|false]   help for show
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
