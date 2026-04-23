## sdt extract

Extract structured items from text input

### Synopsis

Extract specific types of items from plain text using pattern matching.

Supported types: urls, emails, ips, json-blocks, code-blocks, dates

Output is always a JSON array of strings.

```
sdt extract [flags]
```

### Options

```
  -h, --help          help for extract
      --type string   Type of items to extract (urls|emails|ips|json-blocks|code-blocks|dates)
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

