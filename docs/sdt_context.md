## sdt context

Context Tools (context/ work files)

### Synopsis

Manage the agent working files under context/: the documents, their state, and
the derived indexes.

  new · edit · set · unset · get · touch    create a document, edit its frontmatter or body
  rename · rm · clean                       move, retire or discard one
  list · search · map · status · resume     what the corpus holds, and where execution stands
  path · template · docs                    a document path, a type contract, the command docs
  task · check · checklist · close · deprecate
                                            run a plan: checklists, item states, the close
  relations · uid                           typed parent links and document identity
  reindex · lint · memo · todo · wiki · ingest · snapshot · budget
                                            keep the derived surfaces and the registers true

```
sdt context [flags]
```

### Options

```
  -h, --help[=true|false]   help for context
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
* [sdt context budget](sdt_context_budget.md)	 - Report the measured size of the always-loaded context/ surface
* [sdt context check](sdt_context_check.md)	 - Tick a checklist item of a context/ document by its id
* [sdt context checklist](sdt_context_checklist.md)	 - Manage checklist item identifiers
* [sdt context clean](sdt_context_clean.md)	 - Empty the discard areas: context/trash and context/tmp
* [sdt context close](sdt_context_close.md)	 - Close a document in one step (task → plan → analysis)
* [sdt context deprecate](sdt_context_deprecate.md)	 - Retire a document by changing its status, without moving it
* [sdt context docs](sdt_context_docs.md)	 - Generate agent docs in context/sdtdocs/
* [sdt context edit](sdt_context_edit.md)	 - Replace a document body, or one of its sections, from input
* [sdt context get](sdt_context_get.md)	 - Read a document's frontmatter: the raw block or named top-level keys
* [sdt context ingest](sdt_context_ingest.md)	 - Track sources in context/ingestion/ (list, status, archive, plan)
* [sdt context lint](sdt_context_lint.md)	 - Validate context frontmatter and links
* [sdt context list](sdt_context_list.md)	 - List context/ work files
* [sdt context map](sdt_context_map.md)	 - Map the corpus: what it holds, what is awkward, and what it cannot see
* [sdt context memo](sdt_context_memo.md)	 - Manage the planned-operation register (rules and memos)
* [sdt context new](sdt_context_new.md)	 - Create a context/ work file with frontmatter
* [sdt context path](sdt_context_path.md)	 - Print the path for a context/ work file
* [sdt context reindex](sdt_context_reindex.md)	 - Regenerate context/index.md and context/index.json from frontmatter summaries
* [sdt context relations](sdt_context_relations.md)	 - Manage typed parent relations
* [sdt context rename](sdt_context_rename.md)	 - Rename a context/ document (slug change)
* [sdt context resume](sdt_context_resume.md)	 - Show the recorded execution state to resume from (read-only)
* [sdt context rm](sdt_context_rm.md)	 - Discard a document by moving it into context/trash
* [sdt context search](sdt_context_search.md)	 - Search the context/ corpus (section-level, ranked)
* [sdt context set](sdt_context_set.md)	 - Write a top-level frontmatter key with byte fidelity
* [sdt context snapshot](sdt_context_snapshot.md)	 - Manage the derived semantic vector snapshot
* [sdt context status](sdt_context_status.md)	 - Summarize context/ documents per type with next step
* [sdt context task](sdt_context_task.md)	 - Manage plan task checklists
* [sdt context template](sdt_context_template.md)	 - Print the per-type instruction file for a context type
* [sdt context todo](sdt_context_todo.md)	 - Manage the idea inbox (short annotate-only TODOs)
* [sdt context touch](sdt_context_touch.md)	 - Refresh the `updated` timestamp of a context/ document
* [sdt context uid](sdt_context_uid.md)	 - Manage document identifiers (uid)
* [sdt context unset](sdt_context_unset.md)	 - Remove a top-level frontmatter key
* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)
