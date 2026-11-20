## sdt truncate

Truncate text using an approximate token limit

### Synopsis

Truncate input text until the selected tokenizer approximation is at or
below --max-tokens. The count is only an estimate and is not provider-exact.
Text output stays on stdout with the estimate and omission warning on stderr.
JSON and YAML output include the text, estimate, truncation state, omitted
character count, and warning as structured fields.

Strategies:
  chars       Hard cut at character level (default)
  sentences   Cut at the last complete sentence boundary
  sections    Cut at the last complete markdown section boundary

Example:
  cat long_doc.md | sdt truncate --max-tokens 4000
  sdt truncate --max-tokens 2000 --strategy sentences --file essay.txt
  sdt truncate --max-tokens 1000 --strategy sections --model claude --file README.md

```
sdt truncate [flags]
```

### Options

```
  -h, --help[=true|false]   help for truncate
      --max-tokens int      Maximum number of tokens to keep (default 4096)
      --model string        Model name for tokenizer selection (default "gpt-4")
      --strategy string     Truncation strategy: chars|sentences|sections (default "chars")
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
