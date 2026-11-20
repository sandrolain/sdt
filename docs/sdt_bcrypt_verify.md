## sdt bcrypt verify

Verify Bcrypt

### Synopsis

Verify password to match Bcrypt hash

```
sdt bcrypt verify [flags]
```

### Options

```
  -h, --help[=true|false]   help for verify
  -p, --password string     Password to verify
```

### Options inherited from parent commands

```
  -c, --cost int                Bcrypt generation cost (default 14)
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

* [sdt bcrypt](sdt_bcrypt.md)	 - Bcrypt password
