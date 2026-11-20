## sdt bcrypt

Bcrypt password

### Synopsis

Generate Bcrypt hash for password

```
sdt bcrypt [flags]
```

### Options

```
  -c, --cost int            Bcrypt generation cost (default 14)
  -h, --help[=true|false]   help for bcrypt
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
* [sdt bcrypt verify](sdt_bcrypt_verify.md)	 - Verify Bcrypt
