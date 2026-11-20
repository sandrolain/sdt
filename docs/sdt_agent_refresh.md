## sdt agent refresh

Re-render the generated instruction surfaces

### Synopsis

Re-render the generated surfaces from the embedded templates.

  agent refresh    refresh context/instructions/, .agents/skills/ and the
                   AGENTS.md instructions block

It is the narrow alternative to `sdt agent init --force`: it never asks
questions, never rewrites the project identity and never touches .gitignore, the write-once
`<!-- sdt:begin:project -->` block, or the create-only seed files
(context/README.md, topics.yaml, categories.yaml, memo.yaml, todo.yaml,
briefing/onboarding.md). The one key it writes is `instructions.profiles`, and only
when `--with` names a selection to store.

Like `sdt agent init`, it acts on the **current directory**: identity comes
from the .sdt.yaml there, and it does not walk up to a parent project. Run
`sdt agent init` in a directory that has none.

```
sdt agent refresh [flags]
```

### Options

```
  -h, --help[=true|false]   help for refresh
      --target string       Output instruction file (default "AGENTS.md")
      --with stringArray    Optional generation profile to add (repeatable): knowledge, ui, vector, slides, mindmap, diagram, browser, security, roles
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
