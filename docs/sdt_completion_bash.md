## sdt completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(sdt completion bash)

To load completions for every new session, execute once:

#### Linux:

	sdt completion bash > /etc/bash_completion.d/sdt

#### macOS:

	sdt completion bash > $(brew --prefix)/etc/bash_completion.d/sdt

You will need to start a new shell for this setup to take effect.


```
sdt completion bash
```

### Options

```
  -h, --help[=true|false]              help for bash
      --no-descriptions[=true|false]   disable completion descriptions
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

* [sdt completion](sdt_completion.md)	 - Generate the autocompletion script for the specified shell
