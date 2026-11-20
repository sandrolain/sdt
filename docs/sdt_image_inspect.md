## sdt image inspect

Format, dimensions, bytes and alpha of an image

### Synopsis

Report an image's format, dimensions, byte size and whether it carries alpha.

WHEN — a capture or asset needs a validity check: "sane dimensions", the size a
corpus asset is budgeted at, or the format a file actually is rather than the
one its extension claims.

WHEN NOT — a visual claim. Dimensions prove the file decoded, not that the
surface rendered correctly; that is `browser.md`.

RETURNS — one line per fact, or a JSON object with path, format, width, height,
bytes and alpha. Supported decoders: png, jpeg, gif (the standard library's).

EXAMPLE — sdt image inspect context/assets/brand-mark.png

```
sdt image inspect <file> [flags]
```

### Options

```
  -h, --help[=true|false]   help for inspect
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

* [sdt image](sdt_image.md)	 - Image tools
