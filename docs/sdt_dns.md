## sdt dns

DNS lookup for a host

### Synopsis

Perform a DNS lookup for the given host and record type.

Supported record types: A (default), AAAA, MX, TXT, CNAME, NS, PTR

A returns only IPv4 addresses; AAAA returns only IPv6 addresses. Address order
follows the system resolver.

PTR performs a reverse DNS lookup (pass an IP address as --host).

Examples:
  sdt dns --host example.com
  sdt dns --host example.com --type MX
  sdt dns --host example.com --type TXT --format json
  sdt dns --host 8.8.8.8 --type PTR

```
sdt dns [flags]
```

### Options

```
  -h, --help[=true|false]   help for dns
      --host string         Hostname or IP address to look up (required)
      --type string         DNS record type: A|AAAA|MX|TXT|CNAME|NS|PTR (default "A")
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
