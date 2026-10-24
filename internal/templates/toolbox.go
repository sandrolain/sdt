package templates

import "strings"

// ToolboxRow is one line of the Toolbox table that opens the always-loaded
// block: the operation the agent is about to perform, the verb that owns it,
// and the host tool it replaces.
//
// Command, Subcommands and Flags are the machine-readable half. They bind the
// row to the command tree, so TestToolboxRowsExistInManifest fails when a row
// advertises something the binary cannot do — the table is data, not prose, and
// a verb named in a generated surface must exist.
type ToolboxRow struct {
	// Operation is the work, in the agent's words.
	Operation string
	// Display is the markdown cell: the invocation as the agent should type it.
	Display string
	// Command is the top-level command name the row depends on.
	Command string
	// Subcommands are the subcommands the Display names, if any.
	Subcommands []string
	// Flags are the flag names the Display advertises.
	Flags []string
	// Replaces is the markdown cell naming the host tool this makes redundant.
	Replaces string
}

// The command names that appear on more than one row, so the table's data and
// the gate's lookups agree on one spelling.
const (
	toolboxCmdContext = "context"
	toolboxCmdRead    = "read"
)

// ToolboxRows is the ordered table. Order is by how often the operation comes
// up in a session, not by command name. A row is added by the phase that ships
// its verb, so the table is never ahead of the binary.
func ToolboxRows() []ToolboxRow {
	return []ToolboxRow{
		{
			Operation: "corpus, by meaning",
			Display:   "`sdt context search <q>`",
			Command:   toolboxCmdContext,
			Subcommands: []string{
				"search",
			},
			Replaces: "`grep`",
		},
		{
			Operation: "a book, by meaning, without an index",
			Display:   "`sdt chunk search <artifact> <q>`",
			Command:   "chunk",
			Subcommands: []string{
				"search",
			},
			Replaces: "a throwaway search index",
		},
		{
			Operation: "corpus text",
			Display:   "`sdt read -p <ref>`",
			Command:   toolboxCmdRead,
			Replaces:  "`cat`",
		},
		{
			Operation: "line range",
			Display:   "`sdt read --lines a:b`",
			Command:   toolboxCmdRead,
			Flags:     []string{"lines"},
			Replaces:  "`sed -n`",
		},
		{
			Operation: "numbered lines",
			Display:   "`sdt read --numbered`",
			Command:   toolboxCmdRead,
			Flags:     []string{"numbered"},
			Replaces:  "host reader",
		},
		{
			Operation: "listing, sizes",
			Display:   "`sdt ls --stat`",
			Command:   "ls",
			Flags:     []string{"stat"},
			Replaces:  "`ls -la`",
		},
		{
			Operation:   "ingestion queue",
			Display:     "`sdt context ingest list`",
			Command:     toolboxCmdContext,
			Subcommands: []string{"ingest"},
			Replaces:    "`ls` of context/ingestion",
		},
		{
			Operation: "paths by glob",
			Display:   "`sdt find <glob>`",
			Command:   "find",
			Replaces:  "`find`",
		},
		{
			Operation: "tree search",
			Display:   "`sdt grep <p>`",
			Command:   "grep",
			Replaces:  "`grep -r`",
		},
		{
			Operation: "one file",
			Display:   "`sdt grep --path <f>`",
			Command:   "grep",
			Flags:     []string{"path"},
			Replaces:  "`grep -n`",
		},
		{
			Operation: "web page",
			Display:   "`sdt crawldown <url>`",
			Command:   "crawldown",
			Replaces:  "fetcher",
		},
		{
			Operation: "JSON, API",
			Display:   "`sdt http` into `sdt q`",
			Command:   "http",
			Replaces:  "`curl`/`jq`",
		},
		{
			Operation: "two files",
			Display:   "`sdt diff <a> <b>`",
			Command:   "diff",
			Replaces:  "`diff`",
		},
		{
			Operation: "append to corpus",
			Display:   "`sdt context edit --append`",
			Command:   toolboxCmdContext,
			Subcommands: []string{
				"edit",
			},
			Flags:    []string{"append"},
			Replaces: "`cat >>`",
		},
		{
			Operation: "replace one literal",
			Display:   "`sdt edit -p <f> --old/--new`",
			Command:   "edit",
			Flags:     []string{"old", "new"},
			Replaces:  "`python3`/`sed -i`",
		},
		{
			Operation: "document state",
			Display:   "`sdt context check/status/close`",
			Command:   toolboxCmdContext,
			Subcommands: []string{
				"check",
				"status",
				"close",
			},
			Replaces: "none",
		},
	}
}

// ToolboxMarkdown renders the rows as the `### Toolbox` section that opens the
// always-loaded block. The rendering is deterministic: no map iteration, no
// timestamps, so the golden guard stays meaningful.
func ToolboxMarkdown() string {
	var b strings.Builder
	b.WriteString("### Toolbox — what goes through `sdt`\n\n")
	b.WriteString("Pick by the operation; if the verb cannot do it, use the host tool and say so.\n\n")
	b.WriteString("| Need | Verb | Instead of |\n")
	b.WriteString("|---|---|---|\n")
	for _, row := range ToolboxRows() {
		b.WriteString("| ")
		b.WriteString(row.Operation)
		b.WriteString(" | ")
		b.WriteString(row.Display)
		b.WriteString(" | ")
		b.WriteString(row.Replaces)
		b.WriteString(" |\n")
	}
	return b.String()
}
