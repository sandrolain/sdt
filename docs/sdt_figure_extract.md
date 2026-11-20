## sdt figure extract

Extract one source's chapter figures and write their manifest

### Synopsis

Walk a source's images and map them to the converted Markdown's `###### Figure N-M. <caption>`
headings. The mapping is a string match, not a vision call.

An EPUB is walked by its spine: every <img> pairs with its alt text, and the
alt maps to the caption. A PDF carries no alt text and no file paths: its
pairing is purely positional — the converted captions in reading order
against the extracted images in page order. A count mismatch is never
guessed: the default records the unpaired ones as dropped; --strict-position
refuses the ambiguity, naming the counts and the first divergence point.

The images and manifest.json go to <out>/<source-id>/ (default
context/assets/figures/), in the manifest schema the extracted books already
carry. The converted Markdown is found under context/refs/converted/ by the
source's digest, the name doc2md --keep gives it; --converted names it
explicitly.

An image with no matching caption and a caption with no image are both
recorded in the manifest and named on the output, so a silent drop is
impossible. An empty alt is a named finding; its image is paired by position
only when the chapter's counts agree. A repeated caption is a named finding
and its second image keeps a suffixed filename (figure-8-1.1), never an
overwrite. --strict-position fails instead of guessing when a caption's
positional pairing is ambiguous, naming the caption and the candidate
positions.

The images are author-facing: never referenced from wiki, material or a
deliverable. Exit 0 on a complete mapping; dropped figures exit 3 with the
drops recorded in the manifest.

Examples:
  sdt figure extract context/refs/monolithtomicroservices.epub
  sdt figure extract <source> --out /tmp/figs --strict-position

```
sdt figure extract <epub|pdf> [flags]
```

### Options

```
      --converted string               path to the converted Markdown (default: discovered under context/refs/converted/)
  -h, --help[=true|false]              help for extract
      --out string                     output root (default context/assets/figures/)
      --strict-position[=true|false]   fail instead of guessing when positional pairing is ambiguous
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

* [sdt figure](sdt_figure.md)	 - Extract a book's figures with their captions into assets
