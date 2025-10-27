## sdt completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(sdt completion zsh)

To load completions for every new session, execute once:

#### Linux:

	sdt completion zsh > "${fpath[1]}/_sdt"

#### macOS:

	sdt completion zsh > $(brew --prefix)/share/zsh/site-functions/_sdt

You will need to start a new shell for this setup to take effect.


```
sdt completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --file string         Input File
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
```

### SEE ALSO

* [sdt completion](sdt_completion.md)	 - Generate the autocompletion script for the specified shell

