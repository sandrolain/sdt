## sdt material export

Write a clean copy of a guide for a reader

### Synopsis

Resolve one material by its header and write a clean copy under --out.

The copy has no header, no inline marker, no "Verifica" line naming the item bank
and no backticked page slug: a page slug becomes the page's title. The authoring
source is untouched; the copy is a build artefact, not a corpus document. A
concept with several materials is refused: name --syllabus and --lang.

```
sdt material export <concept> [flags]
```

### Options

```
  -h, --help[=true|false]   help for export
      --lang string         language to disambiguate, e.g. it
      --out string          directory the clean copy goes to (required)
      --syllabus string     syllabus to disambiguate, with or without syllabi/
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

* [sdt material](sdt_material.md)	 - Export a study guide without its authoring machinery
