## sdt image diff

Dimension-aware pixel diff of two images

### Synopsis

Compare two images pixel by pixel and report the difference.

WHEN — a screenshot pair (before/after, light/dark, baseline/current) needs a
number and a location, on a machine without a browser driver.

WHEN NOT — a verdict on a render. A pixel delta is a regression tripwire: inspect
the geometry of what moved, never ship on the number alone (`browser.md`).
Use the adopted driver's own diff when it is installed; this command is the path
that does not need one.

RETURNS — the two dimensions (a mismatch is reported and the pixel pass is
skipped), differing pixels, their ratio, the largest per-channel delta, the
changed bounding box, and the threshold used. `--out <file>` writes a PNG with
the differing pixels marked. Supported decoders: png, jpeg, gif.

EXAMPLE — sdt image diff before.png after.png --threshold 8 --out context/tmp/diff.png

```
sdt image diff <a> <b> [flags]
```

### Options

```
  -h, --help[=true|false]   help for diff
      --out string          Write a PNG with the differing pixels marked
      --threshold int       Per-channel difference above which a pixel counts as changed (0-255)
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
