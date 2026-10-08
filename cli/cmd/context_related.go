package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/semantic"
)

// ctxRelatedHint is shown when no semantic snapshot exists yet (the offline
// neighbour surface never loads the model).
const ctxRelatedHint = "no semantic snapshot yet; run `sdt context snapshot build` to build it"

type ctxRelatedHit struct {
	Path    string  `json:"path" yaml:"path"`
	Kind    string  `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status  string  `json:"status,omitempty" yaml:"status,omitempty"`
	Title   string  `json:"title,omitempty" yaml:"title,omitempty"`
	Summary string  `json:"summary,omitempty" yaml:"summary,omitempty"`
	Score   float64 `json:"score" yaml:"score"`
}

var contextRelatedCmd = &cobra.Command{
	Use:   "related <doc>",
	Short: "List the documents semantically nearest to a document",
	Long: `List the documents whose content is nearest to <doc> in the shipped
semantic index, using the persisted vector snapshot (offline, no model load).
The result is advisory and degrades to a message when no snapshot exists.

Examples:
  sdt context related context/wiki/sdt-context-memory.md
  sdt context related notes/2026-01-01-x.md --limit 5 --format json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root, err := os.Getwd()
		exitWithError(cmd, err)
		snap := semantic.LoadSnapshot(root)
		if snap.Empty() {
			outputString(cmd, ctxRelatedHint+"\n")
			return
		}
		path := relatedDocPath(args[0])
		limit := getIntFlag(cmd, "limit", false)
		if limit <= 0 {
			limit = 10
		}
		hits := relatedNeighbors(snap, path, limit)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(map[string]any{ctxFrontmatterResults: hits, ctxMapPath: path}, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(map[string]any{ctxFrontmatterResults: hits, ctxMapPath: path})
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			if len(hits) == 0 {
				outputString(cmd, "no neighbours (document not in the semantic snapshot)\n")
				return
			}
			for _, h := range hits {
				meta := h.Kind
				if h.Status != "" {
					meta += "/" + h.Status
				}
				outputString(cmd, fmt.Sprintf("%-6.3f %-16s %s\n         %s\n", h.Score, meta, h.Path, firstSentence(h.Summary)))
			}
		}
	},
}

// relatedDocPath normalizes a user argument to a context-relative document path
// with the .md extension, matching the snapshot's section ids.
func relatedDocPath(arg string) string {
	p := strings.TrimPrefix(arg, "#")
	p = strings.SplitN(p, "#", 2)[0]
	if !strings.HasSuffix(p, sdtMarkdownExt) {
		p += sdtMarkdownExt
	}
	p = filepathSlash(p)
	if !strings.HasPrefix(p, "context/") {
		p = "context/" + p
	}
	return p
}

// relatedNeighbors enriches the snapshot neighbours of path with each document's
// frontmatter display fields. A neighbour whose file cannot be read keeps its
// path and score.
func relatedNeighbors(snap *semantic.Snapshot, path string, limit int) []ctxRelatedHit {
	neighbors := snap.Neighbors(path, limit)
	out := make([]ctxRelatedHit, 0, len(neighbors))
	for _, n := range neighbors {
		hit := ctxRelatedHit{Path: n.Path, Score: n.Score}
		if data, err := os.ReadFile(n.Path); err == nil { //#nosec G304 -- path derived from the snapshot
			content := string(data)
			hit.Kind = parseFrontmatterField(content, "kind")
			hit.Status = parseFrontmatterField(content, "status")
			hit.Title = parseFrontmatterField(content, "title")
			hit.Summary = parseFrontmatterField(content, "summary")
		}
		out = append(out, hit)
	}
	return out
}

func init() {
	contextCmd.AddCommand(contextRelatedCmd)
	contextRelatedCmd.Flags().Int("limit", 10, "Maximum number of neighbours to list")
}
