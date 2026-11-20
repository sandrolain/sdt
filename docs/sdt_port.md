## sdt port

Check if a TCP port is open on a host

### Synopsis

Attempt a TCP connection to the given host and port.

Reports whether the port is open, and the connection latency.

Examples:
  sdt port --host localhost --port 80
  sdt port --host db.internal --port 5432 --timeout 2s
  sdt port --host example.com --port 443 --format json

```
sdt port [flags]
```

### Options

```
  -h, --help[=true|false]   help for port
      --host string         Hostname or IP address to check (required)
      --port int            TCP port number to check (required)
      --timeout string      Connection timeout (e.g. 2s, 500ms) (default "5s")
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
