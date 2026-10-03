package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxResolveCandidate is one ranked candidate returned by `sdt context resolve`:
// a document of the requested type with the signals the working-context ladder
// uses (status, updated, title, path). It is read-only — resolve never writes.
type ctxResolveCandidate struct {
	Path    string `json:"path" yaml:"path"`
	Type    string `json:"type" yaml:"type"`
	Status  string `json:"status,omitempty" yaml:"status,omitempty"`
	Updated string `json:"updated,omitempty" yaml:"updated,omitempty"`
	Title   string `json:"title,omitempty" yaml:"title,omitempty"`
}

// resolveCandidates reads every document of type t under its directory and
// returns them ranked: a matching --status first, then most-recently updated,
// then path. An empty --status ranks every candidate by recency. The ranking is
// a hint the agent shows the user; it is never a silent decision.
func resolveCandidates(t ctxDocType, status string) ([]ctxResolveCandidate, error) {
	files, err := listContextFiles(t.dir)
	if err != nil {
		return nil, err
	}
	cands := make([]ctxResolveCandidate, 0, len(files))
	for _, path := range files {
		data, rerr := os.ReadFile(path) //#nosec G304 -- context type dir, listed entry
		if rerr != nil {
			continue
		}
		content := string(data)
		cands = append(cands, ctxResolveCandidate{
			Path:    path,
			Type:    t.kind,
			Status:  frontmatterField(content, ctxMapStatus),
			Updated: frontmatterField(content, "updated"),
			Title:   strings.Trim(frontmatterField(content, "title"), `"`),
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		si, sj := cands[i].Status == status, cands[j].Status == status
		if si != sj {
			return si
		}
		if cands[i].Updated != cands[j].Updated {
			return cands[i].Updated > cands[j].Updated
		}
		return cands[i].Path < cands[j].Path
	})
	return cands, nil
}

func outputResolveCandidates(cmd *cobra.Command, cands []ctxResolveCandidate) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(cands, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(cands)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, c := range cands {
			line := c.Path
			if c.Status != "" {
				line += "\t" + c.Status
			}
			if c.Updated != "" {
				line += "\t" + c.Updated
			}
			outputString(cmd, line+"\n")
		}
	}
}

var contextResolveCmd = &cobra.Command{
	Use:   "resolve --type <t> [--status <s>]",
	Short: "Rank candidate documents of a type (read-only helper)",
	Long: `Rank the documents of a type by the signals the working-context resolution
ladder uses — a matching --status first, then most-recently updated — and print
them (path, status, updated).

This is a read-only helper for the workflow commands and the status/doctor hints;
it is deliberately NOT the resolution authority: a workflow command resolves its
subject via the prose ladder in context/commands/index.md and asks the user when
the result is not unique. resolve writes nothing.

Types: ` + ctxListHelpText() + `.

Examples:
  sdt context resolve --type analysis --status active
  sdt context resolve --type plan --status active --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		if typ == ctxDocAliasDecision {
			typ = ctxTypeDecision
		}
		t, ok := ctxTypeLookup(typ)
		if !ok || !t.listSupported {
			exitWithError(cmd, fmt.Errorf("resolve supports type %s, got %q", ctxListHelpText(), typ))
		}
		status := getStringFlag(cmd, "status", false)
		if status != "" {
			if len(t.statuses) == 0 {
				exitWithError(cmd, fmt.Errorf("--status is not supported for --type %s (no status vocabulary)", typ))
			}
			if !ctxStatusInVocab(t, status) {
				exitWithError(cmd, fmt.Errorf("invalid status %q for type %s (use %s)", status, typ, ctxStatusVocab(t)))
			}
		}
		cands, err := resolveCandidates(t, status)
		exitWithError(cmd, err)
		outputResolveCandidates(cmd, cands)
	},
}

func init() {
	contextResolveCmd.Flags().String("type", "", "Document type to resolve candidates for")
	contextResolveCmd.Flags().String("status", "", "Prefer candidates carrying this status (validated against the type vocabulary)")
	contextCmd.AddCommand(contextResolveCmd)
}
