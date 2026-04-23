## sdt prompt validate

Validate a prompt against a maximum token budget

### Synopsis

Render a prompt template and check that it fits within a token budget.

Exits with code 1 if the rendered prompt exceeds --max-tokens.

Example:
  sdt prompt validate --file system.txt --max-tokens 4096 --model gpt-4
  echo "Tell me about {{.topic}}" | sdt prompt validate --vars '{"topic":"Go"}' --max-tokens 2000

```
sdt prompt validate [flags]
```

### Options

```
      --file string        Path to template file
  -h, --help               help for validate
      --max-tokens int     Maximum allowed tokens (default 4096)
      --model string       Model for token counting (default "gpt-4")
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

