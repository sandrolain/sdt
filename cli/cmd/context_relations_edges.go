package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/graph"
	"github.com/spf13/cobra"
)

// ctxRelationsEdgesResult is the payload of `sdt context relations edges`.
// `edges` carries the model's own shape (from, to, family, verb, label), so the
// JSON names the relation the same way every consumer reads it.
type ctxRelationsEdgesResult struct {
	Doc       string       `json:"doc"`
	Direction string       `json:"direction"`
	Depth     int          `json:"depth"`
	Total     int          `json:"total"`
	Edges     []graph.Edge `json:"edges"`
}

var contextRelationsEdgesCmd = &cobra.Command{
	Use:   "edges <doc-ref>",
	Short: "List the recorded edges of a document",
	Long: `List the edges a document records — outbound by default — from the one edge
model every surface reads: ` + "`sources`" + ` (derivation), ` + "`links`" + ` (correlation), the wiki
frontmatter ` + "`relations`" + `, typed body links and the lifecycle parent.

A target the corpus does not serve is not an edge: a ` + "`refs/`" + ` reference, an excluded
path and a wiki relation that does not resolve are left out.

The query answers one hop. ` + "`--depth`" + ` accepts 1; transitive expansion is a
separate wave and is refused rather than silently not done.

Examples:
  sdt context relations edges plan/<plan-file>
  sdt context relations edges wiki/architecture/deployment-pipeline --inbound
  sdt context relations edges analysis/<file> --verb depends_on --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)

		depth := getIntFlag(cmd, "depth", false)
		if depth != 1 {
			exitWithError(cmd, fmt.Errorf("--depth %d: the edge model answers one hop; transitive expansion is not part of this query", depth))
		}

		root := sdtProjectRoot()
		if root == "" {
			root = "."
		}
		g, err := graph.Build(root)
		exitWithError(cmd, err)

		inbound := getBoolFlag(cmd, "inbound", false)
		verb := getStringFlag(cmd, "verb", false)
		var edges []graph.Edge
		if inbound {
			edges = g.Inbound(doc.Path, verb)
		} else {
			edges = g.Outbound(doc.Path, verb)
		}
		outputRelationsEdges(cmd, doc.Path, inbound, depth, edges)
	},
}

func outputRelationsEdges(cmd *cobra.Command, docPath string, inbound bool, depth int, edges []graph.Edge) {
	direction := "outbound"
	if inbound {
		direction = "inbound"
	}
	res := ctxRelationsEdgesResult{
		Doc:       docPath,
		Direction: direction,
		Depth:     depth,
		Total:     len(edges),
		Edges:     edges,
	}
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		if len(edges) == 0 {
			outputString(cmd, fmt.Sprintf("no %s edge for %s\n", direction, docPath))
			return
		}
		for _, e := range edges {
			other := e.To
			if inbound {
				other = e.From
			}
			verb := e.Verb
			if verb == "" {
				verb = "-"
			}
			line := fmt.Sprintf("%-12s %-14s %s", e.Family, verb, other)
			if e.Label != "" {
				line += "  (" + e.Label + ")"
			}
			outputString(cmd, line+"\n")
		}
	}
}
