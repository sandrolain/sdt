## sdt agent section

Manage tagged sections in AGENTS.md

### Synopsis

Manage tagged sections inside an instruction file (default AGENTS.md).

Sections are delimited by:

  <!-- sdt:begin:NAME -->
  ...content...
  <!-- sdt:end:NAME -->

Section content is read from extra arguments, stdin, --input or --file.

Examples:
  sdt agent section list
  sdt agent section show workflow
  echo "make test" | sdt agent section add commands
  sdt agent section update commands --file commands.md
  sdt agent section remove commands

### Options

```
  -h, --help            help for section
      --target string   Instruction file to manage (default "AGENTS.md")
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

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, sections, guides)
* [sdt agent section add](sdt_agent_section_add.md)	 - Add a new tagged section
* [sdt agent section list](sdt_agent_section_list.md)	 - List tagged sections
* [sdt agent section remove](sdt_agent_section_remove.md)	 - Remove a tagged section
* [sdt agent section set](sdt_agent_section_set.md)	 - Add or update a tagged section
* [sdt agent section show](sdt_agent_section_show.md)	 - Show a tagged section content
* [sdt agent section update](sdt_agent_section_update.md)	 - Update an existing tagged section

