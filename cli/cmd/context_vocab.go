package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// contextVocabCmd prints the per-kind status vocabulary the validators enforce.
//
// The vocabulary is declared as the single source of truth in `contextTypeList`
// (`context_types.go`), and until this verb nothing could print it: it was read
// by the validators and the lint's error message, while the one reader-facing
// copy — the `## Statuses` table in `context/architecture/stack.md` — was a
// hand-maintained mirror with nothing linking the two. An agent that needed the
// vocabulary grepped that prose.
type ctxVocabRow struct {
	Kind      string   `json:"kind" yaml:"kind"`
	Tier      string   `json:"tier,omitempty" yaml:"tier,omitempty"`
	Directory string   `json:"directory,omitempty" yaml:"directory,omitempty"`
	Default   string   `json:"default,omitempty" yaml:"default,omitempty"`
	Statuses  []string `json:"statuses" yaml:"statuses"`
}

// ctxVocabRows reads the registry in its declared order, so the output is
// deterministic and matches the order the type list is maintained in. A type
// with no status vocabulary is skipped: it has nothing to report here.
func ctxVocabRows() []ctxVocabRow {
	rows := make([]ctxVocabRow, 0, len(contextTypeList))
	for _, t := range contextTypeList {
		if len(t.statuses) == 0 {
			continue
		}
		statuses := make([]string, len(t.statuses))
		copy(statuses, t.statuses)
		rows = append(rows, ctxVocabRow{
			Kind:      t.kind,
			Tier:      t.tier,
			Directory: t.dir,
			Default:   t.defaultStatus,
			Statuses:  statuses,
		})
	}
	return rows
}

// ctxVocabRenderText prints one block per type: the vocabulary, with the default
// marked, so a reader can tell which value a new document gets without opening
// the registry.
func ctxVocabRenderText(rows []ctxVocabRow) string {
	var b strings.Builder
	b.WriteString("per-kind status vocabulary (declared in the type registry)\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s\n", r.Kind)
		for _, s := range r.Statuses {
			marker := " "
			if s == r.Default {
				marker = "*"
			}
			fmt.Fprintf(&b, "    %s %s\n", marker, s)
		}
		if r.Default != "" {
			fmt.Fprintf(&b, "      * = default for a new %s\n", r.Kind)
		}
	}
	b.WriteString("\n  A type absent here carries no status field.\n")
	return b.String()
}

var contextVocabCmd = &cobra.Command{
	Use:   "vocab",
	Short: "Print the per-kind status vocabulary",
	Long: `Print the status vocabulary of every document type that carries one, with
the default a new document gets.

WHEN — you are about to set, filter or reason about a document's status and need
the values the validator will accept, rather than a guess or a recollection of
the prose table.

WHEN NOT — you want one document's current status (sdt context status get), or
the per-type document counts (sdt context status). This verb describes what the
vocabulary **is**, not what any document holds.

RETURNS — one block per type: every accepted value in registry order, with the
default marked, plus the type's directory and tier. Both the accepted values and
the default are read from the same registry the validators use, so the answer
cannot drift from what the CLI enforces.

EXAMPLE — sdt context vocab
          sdt context vocab --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rows := ctxVocabRows()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(rows)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, ctxVocabRenderText(rows))
		}
	},
}
