## sdt cert inspect

Inspect a TLS certificate and show structured details

### Synopsis

Inspect a TLS certificate from a live host or a PEM file.

Source priority:
  1. --host host[:port]   fetch live certificate via TLS
  2. --file path          read PEM file
  3. stdin                read PEM from stdin

Examples:
  sdt cert inspect --host example.com
  sdt cert inspect --host example.com:8443 --format json
  sdt cert inspect --file cert.pem
  cat cert.pem | sdt cert inspect

```
sdt cert inspect [flags]
```

### Options

```
      --file string   Path to PEM certificate file
  -h, --help          help for inspect
      --host string   Host (or host:port) to fetch certificate from
      --insecure      Skip TLS certificate verification
```

### Options inherited from parent commands

```
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt cert](sdt_cert.md)	 - Inspect TLS/X.509 certificates

