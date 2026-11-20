## sdt cert

Inspect TLS/X.509 certificates

### Synopsis

Inspect TLS or X.509 PEM certificates.

Subcommands:
  inspect   Inspect a certificate (from host or PEM file)
  expiry    Show only expiry information

### Options

```
  -h, --help[=true|false]   help for cert
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
* [sdt cert expiry](sdt_cert_expiry.md)	 - Show certificate expiry information
* [sdt cert inspect](sdt_cert_inspect.md)	 - Inspect a TLS certificate and show structured details
