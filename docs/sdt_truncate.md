## sdt truncate

Truncate text to a maximum number of LLM tokens

### Synopsis

Truncate input text so that it fits within a maximum token budget.

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
  -h, --help              help for truncate
      --max-tokens int    Maximum number of tokens to keep (default 4096)
      --model string      Model name for tokenizer selection (default "gpt-4")
      --strategy string   Truncation strategy: chars|sentences|sections (default "chars")
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

