## sdt password

Generate password

### Synopsis

Generate password

```
sdt password [flags]
```

### Options

```
  -d, --digits int             Digits number (default -1)
  -h, --help[=true|false]      help for password
  -l, --length int             Password length (default 16)
  -n, --number int             Number of passwords (default 1)
  -r, --repeat[=true|false]    Allow repeated characters
  -s, --symbols int            Symbols number (default -1)
  -u, --untyped[=true|false]   Only lowercase characters
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

* [sdt](sdt.md)	 - Smart Developer Tools
