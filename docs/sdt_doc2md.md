## sdt doc2md

Convert documents to Markdown with the first runnable converter

### Synopsis

Convert one or more local documents to GitHub-Flavored Markdown.

The converter chain is tried in order: anydoc, docling, markitdown. A tool
counts as available only when it launches (an exec probe, never a PATH lookup),
and a tool that fails, times out or produces no markdown advances the chain.

Markdown goes to stdout to read. --keep stores the document under
context/refs/converted/<slug>-<sha256[:12]>.md with provenance frontmatter
and prints the path, then the routing counts the ingestion doctrine asks for —
section-level headings (markdown levels 2 and 3 of the conversion) and the
conversion's lines — so the receipt is measured here, never by hand;
--output-file writes the same stamped document to an
explicit path. A re-keep of an already kept source that would produce different
bytes is refused (the citing documents' digests are named), because that rewrite
is what makes refs/converted/<file>@<sha> citations stale; --force is the
explicit way through. Exit codes are normalized:
  0 success · 1 failed · 2 no runnable converter · 3 PDF needs OCR

This command replaces the removed context/scripts/convert-to-md.sh wrapper
(the old invocation was "context/scripts/convert-to-md.sh <file> [outdir]").

```
sdt doc2md <file>... [flags]
```

### Options

```
      --dir string           Convert every supported document under this directory
      --force[=true|false]   Allow a rewrite of an existing kept artifact whose bytes would change (the citing documents are named either way)
      --glob string          With --dir, convert only the documents matching this glob (the sdt find dialect)
  -h, --help[=true|false]    help for doc2md
      --keep[=true|false]    Keep the document under context/refs/converted/ with provenance frontmatter
      --name string          Override the kept filename stem (with --keep)
      --output-file string   Write the document to an explicit path instead of stdout
      --timeout duration     Per-process timeout (default 2m0s)
      --tool string          Pin one converter (anydoc, docling, markitdown)
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

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt doc2md list](sdt_doc2md_list.md)	 - List the kept converted documents
* [sdt doc2md reindex](sdt_doc2md_reindex.md)	 - Regenerate the converted-corpus index projections
* [sdt doc2md set](sdt_doc2md_set.md)	 - Enrich a kept document's summary and links
* [sdt doc2md tools](sdt_doc2md_tools.md)	 - Probe the converter chain and print which tools are runnable
