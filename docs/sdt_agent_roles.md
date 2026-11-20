## sdt agent roles

Role register and generated role profiles

### Synopsis

Manage the closed role register and the generated role profiles under
`context/roles/`:

  roles show    list the closed role register
  roles init    generate the role profiles under context/roles/
  roles check   validate the profile set, drift and owned-path overlap


### Options

```
  -h, --help[=true|false]   help for roles
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)
* [sdt agent roles check](sdt_agent_roles_check.md)	 - Validate the role profiles deterministically
* [sdt agent roles init](sdt_agent_roles_init.md)	 - Generate role profiles under context/roles/
* [sdt agent roles show](sdt_agent_roles_show.md)	 - List the closed role register
