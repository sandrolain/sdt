## sdt context wiki

Knowledge graph operations (wiki lint)

### Synopsis

Knowledge-pipeline commands for the wiki/ knowledge graph under context/.

  lint       recursive schema/graph validation plus the source markers
  units      the heading/unit spans of an artifact
  coverage   the citation coverage record and receipt block
  inventory  concept-inventory counters, rows and scope
  claims     broad citation spans
  reference  the conditional reference bridge
  frontier   the read-only graph frontier score
  orphans    concept pages with no relation and no index link
  comprehension  the two-page-per-book sample
  audit      the generated receipt with page/claim and inventory totals
  receipt    receipt lint

### Options

```
  -h, --help[=true|false]   help for wiki
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
* [sdt context wiki audit](sdt_context_wiki_audit.md)	 - Print the receipt block, page/claim totals and inventory status
* [sdt context wiki claims](sdt_context_wiki_claims.md)	 - Flag broad claims that cite too many lines or units
* [sdt context wiki comprehension](sdt_context_wiki_comprehension.md)	 - Select the two-page-per-book comprehension sample
* [sdt context wiki coverage](sdt_context_wiki_coverage.md)	 - Classify an artifact's units from the pages' citations (OK/PART/GAP)
* [sdt context wiki frontier](sdt_context_wiki_frontier.md)	 - Report where the wiki is thin (read-only frontier score)
* [sdt context wiki glossary](sdt_context_wiki_glossary.md)	 - List the claim-backed terms the corpus defines (read-only)
* [sdt context wiki inventory](sdt_context_wiki_inventory.md)	 - Validate concept-inventory notes (counters, rows, claims, scope)
* [sdt context wiki lint](sdt_context_wiki_lint.md)	 - Validate the wiki knowledge graph and source markers
* [sdt context wiki orphans](sdt_context_wiki_orphans.md)	 - List concept pages with no typed relation and no index link
* [sdt context wiki receipt](sdt_context_wiki_receipt.md)	 - Receipt operations (lint a note's receipt block)
* [sdt context wiki reference](sdt_context_wiki_reference.md)	 - Check the conditional reference→index bridge
* [sdt context wiki render](sdt_context_wiki_render.md)	 - Report the freshness of the derived lane, or gate on it
* [sdt context wiki units](sdt_context_wiki_units.md)	 - Enumerate a converted artifact's headings (units) and spans
