## sdt completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	sdt completion fish | source

To load completions for every new session, execute once:

	sdt completion fish > ~/.config/fish/completions/sdt.fish

You will need to start a new shell for this setup to take effect.


```
sdt completion fish [flags]
```

### Options

```
  -h, --help              help for fish
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

