## sdt memory

Persistent key-value memory store for AI agents

### Synopsis

Manage a persistent key-value memory store backed by SQLite.

Entries are scoped by project (and optionally group). Project and group are
resolved from --project/--group flags, or from .sdt.yaml discovered by
walking up from the current directory.

### Options

```
  -h, --help   help for memory
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt memory delete](sdt_memory_delete.md)	 - Delete a memory entry (or all entries for the project)
* [sdt memory export](sdt_memory_export.md)	 - Export memory entries as JSON
* [sdt memory get](sdt_memory_get.md)	 - Retrieve a value by key
* [sdt memory groups](sdt_memory_groups.md)	 - List all known groups
* [sdt memory import](sdt_memory_import.md)	 - Import memory entries from JSON (stdin or --file)
* [sdt memory init](sdt_memory_init.md)	 - Create .sdt.yaml in the current directory
* [sdt memory list](sdt_memory_list.md)	 - List memory entries for a project
* [sdt memory projects](sdt_memory_projects.md)	 - List all known projects
* [sdt memory search](sdt_memory_search.md)	 - Full-text search across memory entries
* [sdt memory set](sdt_memory_set.md)	 - Store a key-value entry

