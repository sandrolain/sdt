## sdt context wiki glossary

List the claim-backed terms the corpus defines (read-only)

### Synopsis

List every term a page defines in its `## Terms` section, with the claim that
defines it, the claim's digest and the span the definition restates.

WHEN — you need to know whether the corpus defines a term, or which claim a definition
rests on, before writing material or citing one.
WHEN NOT — you want the page a term belongs to: open the page; the glossary is an
index, not a reading.

RETURNS — one row per entry: the term, its page, the claim id and anchor, the digest
that moves when the wording does, and the source span. An entry the lint would report
as malformed is left out rather than listed half-formed, so the glossary and
`sdt context wiki lint` never disagree. Derived and read-only: it holds no file
and no status, so it cannot go stale.

With `--aliases` it answers the other question — which page a concept is called X
by another name — from the `aliases` frontmatter field. Same verb, same rows, same
read-only source; the default output is unchanged.

EXAMPLE — sdt context wiki glossary
          sdt context wiki glossary --aliases
          sdt context wiki glossary --format json

```
sdt context wiki glossary [flags]
```

### Options

```
      --aliases[=true|false]   List the alias → page rows instead of the claim-backed terms
  -h, --help[=true|false]      help for glossary
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

* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)
