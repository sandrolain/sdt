## sdt ipinfo

Classify an IP locally or look up an explicit target

### Synopsis

Classify an IP address locally with --local, or retrieve provider-backed
geolocation for an explicit IP address or hostname. No target is inferred from
the machine's public egress address.

Examples:
  sdt ipinfo --local --ip 192.168.1.1
  sdt ipinfo --ip 203.0.113.7 --format json

```
sdt ipinfo [flags]
```

### Options

```
  -h, --help[=true|false]    help for ipinfo
  -t, --ip string            Target IP address or hostname (required for geolocation)
      --local[=true|false]   Classify a literal IP locally without a provider request
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
