## sdt context get

Read a document's frontmatter: the raw block or named top-level keys

### Synopsis

Read the frontmatter of a context document. The document is resolved like
every verb that takes a <doc>, in this order: a path under context/, a bare file
name, a wiki page id or file name, or a uid; plus --type/--slug identity flags.

With no key, the raw frontmatter block (including its --- fences) is printed
byte-exact. With one or more keys, one value is printed per requested key, in
the order asked.

Only top-level keys are addressable: a dotted/nested key is rejected with a
named error rather than returning an empty value. A missing key prints an
empty line on stdout, a note on stderr, and exits 1. Under --format json|yaml
the result is a mapping, so a missing key is an explicit null.

Examples:
  sdt context get plan/<plan-file>        # the raw block
  sdt context get plan/<plan-file> status # one value
  sdt context get --type analysis --slug x objective title

```
sdt context get <doc> [<key>...] [flags]
```

### Options

```
  -h, --help[=true|false]   help for get
      --number string       Decision number (type decision)
      --phase string        Phase for a task file (type tasks)
      --plan string         Plan reference for a task file (type tasks)
      --slug string         Slug for identity resolution
      --stream string       Split-file label for a task file (type tasks)
      --type string         Document type for identity resolution
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

* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
