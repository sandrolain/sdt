## sdt sha1

SHA-1

### Synopsis

Generate SHA-1.

`--hex` prints the hexadecimal form (the `shasum` shape); the default is
the raw digest bytes.

```
sdt sha1 [flags]
```

### Options

```
  -h, --help[=true|false]   help for sha1
      --hex[=true|false]    Print the hexadecimal digest (the shasum shape) instead of the raw bytes
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
