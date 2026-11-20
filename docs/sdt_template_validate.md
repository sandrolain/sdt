## sdt template validate

Check a rendered template against an estimated token limit

### Synopsis

Render the selected template and data, then compare the approximate token
count with --max-tokens. JSON and YAML output include the validation result and
the estimate limitation. Exits with code 1 when the estimate exceeds the limit.

```
sdt template validate [flags]
```

### Options

```
  -h, --help[=true|false]   help for validate
      --max-tokens int      Maximum estimated tokens for the rendered template (default 4096)
      --model string        Model used to select a token estimator (default "gpt-4")
```

### Options inherited from parent commands

```
      --data string             Inline data as JSON or YAML
      --file string             Input File
      --file-data string        Path to JSON or YAML data file
      --format string           Output format: text|json|yaml (default "text")
      --inb64 bytesBase64       Input Base 64
      --input string            Input String
      --log-format string       Log format: text|json (default "text")
      --no-color[=true|false]   Disable ANSI color codes
      --no-env[=true|false]     Do not load a project-root .env file
      --progress string         Progress narration on stderr: auto|human|json|none (default "auto")
      --quiet[=true|false]      Suppress informational messages, only output result
      --template-file string    Path to a Go text/template file
      --tmpl string             Template string (Go text/template syntax)
```

### SEE ALSO

* [sdt template](sdt_template.md)	 - Render a Go text/template with JSON or YAML data
