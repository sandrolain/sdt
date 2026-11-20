## sdt template

Render a Go text/template with JSON or YAML data

### Synopsis

Render a Go text/template using JSON or YAML data.

Choose one template source: --tmpl, --template-file, --file, --input, --inb64,
positional input, or stdin.

Choose at most one variable source: --data (inline JSON or YAML) or
--file-data (a JSON or YAML file). Stdin is the template source, never a
variable source.

Example:
  sdt template --template-file greeting.tmpl --file-data vars.yaml

```
sdt template [flags]
```

### Options

```
      --data string            Inline data as JSON or YAML
      --file-data string       Path to JSON or YAML data file
  -h, --help[=true|false]      help for template
      --template-file string   Path to a Go text/template file
      --tmpl string            Template string (Go text/template syntax)
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
* [sdt template validate](sdt_template_validate.md)	 - Check a rendered template against an estimated token limit
