## sdt cert expiry

Show certificate expiry information

### Synopsis

Show only expiry date and days remaining for a certificate.

Examples:
  sdt cert expiry --host example.com
  sdt cert expiry --host example.com:443 --format json
  sdt cert expiry --file cert.pem

```
sdt cert expiry [flags]
```

### Options

```
      --file string             Path to PEM certificate file
  -h, --help[=true|false]       help for expiry
      --host string             Host (or host:port) to fetch certificate from
      --insecure[=true|false]   Skip TLS certificate verification
```

### Options inherited from parent commands

```
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

* [sdt cert](sdt_cert.md)	 - Inspect TLS/X.509 certificates
