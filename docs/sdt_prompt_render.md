## sdt prompt render

Render a prompt template with variables

### Synopsis

Render a Go text/template prompt with JSON or YAML variables.

Template source priority:
  1. --template flag (inline string)
  2. --file flag (path to template file)
  3. stdin

Variable source:
  1. --vars flag (inline JSON or YAML)
  2. --vars-file flag (path to JSON or YAML file)

Example:
  sdt prompt render --template "You are {{.role}}." --vars '{"role":"assistant"}'
  sdt prompt render --file system.txt --vars-file context.json

```
sdt prompt render [flags]
```

### Options

```
      --file string        Path to template file
  -h, --help               help for render
      --model string       Model for token counting (used with --show-tokens) (default "gpt-4")
      --show-tokens        Include token count in output
      --template string    Inline template string
      --vars string        Variables as inline JSON or YAML
      --vars-file string   Path to JSON or YAML variables file
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

* [sdt prompt](sdt_prompt.md)	 - Manage and render LLM prompt templates

