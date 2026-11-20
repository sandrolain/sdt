## sdt doc2md tools

Probe the converter chain and print which tools are runnable

### Synopsis

Probe every converter in the chain (anydoc, docling, markitdown) and print
whether it launches, plus its version when it reports one.

A tool is available only if it launches; presence on PATH is not enough.

```
sdt doc2md tools [flags]
```

### Options

```
  -h, --help[=true|false]   help for tools
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
      --timeout duration        Per-process timeout (default 2m0s)
```

### SEE ALSO

* [sdt doc2md](sdt_doc2md.md)	 - Convert documents to Markdown with the first runnable converter
