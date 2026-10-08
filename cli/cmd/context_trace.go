package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/ctxrel"
)

// ── context trace (sdt context trace) ─────────────────────────────────────────
//
// A read-only view of a document's lifecycle chain, leaf → root, assembled from
// the recorded typed relations (internal/ctxrel: task→plan, plan→analysis) — the
// orientation view the openrig chain-file convention asks for. Never from
// memory; deterministic; writes nothing.

// ctxTraceStep is one step of the chain.
type ctxTraceStep struct {
	Ref    string `json:"ref" yaml:"ref"`
	Kind   string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Title  string `json:"title,omitempty" yaml:"title,omitempty"`
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
}

// ctxTraceStepFor reads the kind/title/status of a corpus-relative reference.
func ctxTraceStepFor(ref string) ctxTraceStep {
	step := ctxTraceStep{Ref: ref}
	data, err := os.ReadFile(filepath.FromSlash(ref)) //#nosec G304 -- corpus-relative path
	if err != nil {
		return step
	}
	fm, _ := contextwiki.SplitFrontmatter(string(data))
	step.Kind = contextwiki.FrontmatterField(fm, "kind")
	step.Title = contextwiki.FrontmatterField(fm, "title")
	step.Status = contextwiki.FrontmatterField(fm, "status")
	return step
}

// ctxTraceChain walks the parent relations from ref to the root. A cycle stops
// the walk (a malformed relation loop is reported, never hung on); a missing
// parent ends the chain.
func ctxTraceChain(ref string, edges *ctxrel.Edges) []ctxTraceStep {
	var chain []ctxTraceStep
	seen := map[string]bool{}
	for ref != "" && !seen[ref] {
		seen[ref] = true
		chain = append(chain, ctxTraceStepFor(ref))
		ref = edges.ParentOf(ref)
	}
	return chain
}

var contextTraceCmd = &cobra.Command{
	Use:   "trace <doc-ref>",
	Short: "Show a document's lifecycle chain (leaf → root)",
	Long: `Show a document's lifecycle chain, from the document up to its root, over the
recorded typed relations (task → plan → analysis). Read-only and deterministic:
the chain is assembled from what is recorded, never from memory. A document with
no parent is its own chain.

Examples:
  sdt context trace context/tasks/<task-file>.md
  sdt context trace plan/<plan-file> --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		edges, err := ctxrel.Load(sdtWorkDir)
		exitWithError(cmd, err)
		chain := ctxTraceChain(filepath.ToSlash(doc.Path), edges)
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(chain, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(chain)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			for i, s := range chain {
				line := fmt.Sprintf("%s%s  %s", strings.Repeat("  ", i), s.Ref, s.Kind)
				if s.Title != "" {
					line += "  — " + s.Title
				}
				outputString(cmd, line+"\n")
			}
		}
	},
}

func init() {
	contextCmd.AddCommand(contextTraceCmd)
}
