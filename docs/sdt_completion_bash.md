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

	sdt completion bash > /usr/local/etc/bash_completion.d/sdt

You will need to start a new shell for this setup to take effect.


```
sdt completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -f, --file string   Input File
  -i, --input         Input Prompt
```

### SEE ALSO

* [sdt completion](sdt_completion.md)	 - Generate the autocompletion script for the specified shell

