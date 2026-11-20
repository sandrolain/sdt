## sdt tokens

Count approximate LLM tokens for the given text

### Synopsis

Count approximate LLM tokens for the given text.

Uses a regex approximation of the cl100k_base (GPT-4 / Claude) or p50k_base
(GPT-2) tokenizer without requiring vocabulary data files. Counts are estimates,
not provider-exact token counts. Unknown model names use the cl100k approximation
and are identified in the output.

Supported model aliases (used to select tokenizer family):
  gpt-4, gpt-4o, gpt-3.5, gpt-3.5-turbo   → cl100k_base
  claude, claude-3, gemini, mistral         → cl100k_base
  gpt-2                                     → p50k_base
  llama, llama-2, llama-3                   → llama (cl100k + 5%)

Examples:
  echo "Hello, world!" | sdt tokens
  sdt tokens --model gpt-4 --file prompt.txt
  sdt tokens --model claude --format json

```
sdt tokens [flags]
```

### Options

```
  -h, --help[=true|false]   help for tokens
      --model string        Model name to select tokenizer family (gpt-4, gpt-3.5, claude, llama, gpt-2, ...) (default "gpt-4")
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
