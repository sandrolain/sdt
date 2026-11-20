## sdt extract

Extract structured items from text input

### Synopsis

Extract specific types of items from plain text using pattern matching and structural parsing.

Supported types: urls, emails, ips, json-blocks, code-blocks, dates.
JSON extraction returns complete outer object/array values and errors on malformed values.
Markdown extraction returns complete fenced block bodies without fence markers; unterminated fences are ignored.

Output is always a JSON array of strings.

```
sdt extract [flags]
```

### Options

```
  -h, --help[=true|false]   help for extract
      --type string         Type of items to extract (urls|emails|ips|json-blocks|code-blocks|dates)
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
