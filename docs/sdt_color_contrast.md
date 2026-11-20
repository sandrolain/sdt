## sdt color contrast

WCAG 2.x contrast ratio of two colour values

### Synopsis

Report the WCAG 2.x contrast ratio of two colour values.

WHEN — you are choosing or checking token values and need the ratio before a
render exists: a colour pair in a token file, a proposed palette, a review claim
about text on a surface.

WHEN NOT — a rendered surface. Opacity, gradients, blending and images change
what the user sees; the ratio here is computed from the two values you pass, and
the rendered check still belongs to `browser.md`.

RETURNS — the two normalised colours and their relative luminance, the contrast
ratio, and the five WCAG 2.x thresholds (AA/AAA, normal/large text, non-text
3.0). `--format json|yaml` gives the same as one object. The exit status is 0
for any valid pair: a failing ratio is a finding, not a command error.

Accepted input: #rgb, #rrggbb, rgb(r, g, b), rgba(r, g, b, 1). An alpha below 1
is refused, because the composited colour is the one that has a ratio.

EXAMPLE — sdt color contrast '#777777' '#ffffff'

```
sdt color contrast <fg> <bg> [flags]
```

### Options

```
  -h, --help[=true|false]   help for contrast
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

* [sdt color](sdt_color.md)	 - Colour tools
