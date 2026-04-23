## sdt template

Render a Go text/template with JSON or YAML data

### Synopsis

Render a Go text/template using JSON or YAML data.

Template source (in priority order):
  1. --tmpl flag
  2. stdin (when --data or --file-data is provided)

Data source (in priority order):
  1. --data flag (inline JSON or YAML string)
  2. --file-data flag (path to JSON or YAML file)
  3. No data (template receives nil)

Example:
  echo '{"name":"Alice"}' | sdt template --tmpl "Hello, {{.name}}!"

```
sdt template [flags]
```

### Options

```
      --data string        Inline data as JSON or YAML
      --file-data string   Path to JSON or YAML data file
  -h, --help               help for template
      --tmpl string        Template string (Go text/template syntax)
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools

