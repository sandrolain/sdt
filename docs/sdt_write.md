## sdt write

Write bytes to a file

### Synopsis

Write input bytes to a file.

WHEN — a whole file is being written: a fixture, a derived render, bytes the
host writer would have written. The input arrives via stdin, `--input`, `--file`
or `--inb64`; `--multi` reads a JSON array and writes one content per comma-
separated path.

WHEN NOT — a surgical change: `sdt edit` replaces a guarded literal and
preserves the mode; this verb replaces the whole file. A corpus document
appends through `sdt context edit --append`.

RETURNS — the written paths, one per line. Without `--overwrite` an existing
file is refused, so the verb never destroys content silently.

`--append` is the `cat >>` shape: the input is added after the file's
current bytes, the file is created when it does not exist (mode 0644), an
existing file keeps its mode, and a directory or a corpus document is refused
with the named reason. It conflicts with `--overwrite`: appending and
replacing are two different asks.

EXAMPLE — `sdt context new --type notes … | sdt write -p out.md`,
          `echo tail | sdt write -p fixture.txt --append`

```
sdt write [flags]
```

### Examples

```
  sdt write -p out.md < payload.txt
  sdt write -p out.md < payload.txt --overwrite
  echo tail | sdt write -p fixture.txt --append
```

### Options

```
      --append[=true|false]      Add the input after the file's current bytes (creates the file when absent)
  -b, --binary[=true|false]      Input as Base64 encoded content
  -p, --file string              File path
  -h, --help[=true|false]        help for write
  -m, --multi[=true|false]       Input as JSON array with multiple contents
  -o, --overwrite[=true|false]   Overwrite if already exist
```

### Options inherited from parent commands

```
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
