package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// researchRoot returns the project root (the directory holding .sdt.yaml), or
// the current directory when no project config is found. The run area is
// context/tmp/research under it.
func researchRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, sdtConfigFile)); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir, nil
		}
		dir = parent
	}
}

// resolveRun loads the run named by --run, or the latest run when it is empty.
func resolveRun(cmd *cobra.Command) (*research.Run, string, error) {
	root, err := researchRoot()
	if err != nil {
		return nil, "", err
	}
	runID := getStringFlag(cmd, "run", false)
	if runID == "" {
		runID, err = research.FindLatest(root)
		if err != nil {
			return nil, "", err
		}
	}
	if runID == "" {
		return nil, "", errors.New("no research run found; create one with `sdt research init --query \"…\"`")
	}
	r, err := research.Load(root, runID)
	if err != nil {
		return nil, "", fmt.Errorf("load run %s: %w", runID, err)
	}
	return r, root, nil
}

// outputResearch marshals a value in the requested --format.
func outputResearch(cmd *cobra.Command, v any, text func() string) {
	switch getFormat(cmd) {
	case formatJSON:
		out, err := json.MarshalIndent(v, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(out)+"\n")
	case formatYAML:
		out, err := yaml.Marshal(v)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, text())
	}
}

var researchCmd = &cobra.Command{
	Use:   "research",
	Short: "Auditable research runs: manifest, provenance and resume",
	Long: `Manage research runs with an auditable YAML manifest.

A run lives under context/tmp/research/<run-id>/ and records its query, budget,
checkpoint and one entry per source (canonical URL, provenance, status). The
content acquisition and verification stages are added by later commands; this
group creates a run and inspects it.`,
}

// ── research init ─────────────────────────────────────────────────────────────

var researchInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a research run",
	Long: `Create a new research run under context/tmp/research/<run-id>/ with its
manifest and provenance sidecar. --query is the question the run answers;
--scope narrows it. The run id is printed (or the manifest in --format
json|yaml).

Examples:
  sdt research init --query "which embedded vector store fits SDT"
  sdt research init --query "crawldown options" --scope "capture flags only"`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		query := getStringFlag(cmd, "query", true)
		scope := getStringFlag(cmd, "scope", false)
		root, err := researchRoot()
		exitWithError(cmd, err)

		r := research.NewRun(query, scope, time.Now())
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}
		outputResearch(cmd, r, func() string { return r.RunID + "\n" })
	},
}

// ── research inspect ──────────────────────────────────────────────────────────

// researchInspectView is the machine-readable shape of `research inspect`.
type researchInspectView struct {
	RunID      string              `json:"run_id" yaml:"run_id"`
	Query      string              `json:"query" yaml:"query"`
	Scope      string              `json:"scope,omitempty" yaml:"scope,omitempty"`
	Created    string              `json:"created" yaml:"created"`
	Updated    string              `json:"updated" yaml:"updated"`
	Budget     research.Budget     `json:"budget,omitempty" yaml:"budget,omitempty"`
	Checkpoint research.Checkpoint `json:"checkpoint,omitempty" yaml:"checkpoint,omitempty"`
	Sources    []research.Source   `json:"sources" yaml:"sources"`
	Counts     map[string]int      `json:"counts" yaml:"counts"`
}

var researchInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Show a research run: sources, status, budget and checkpoint",
	Long: `Show the recorded state of a research run without changing it. --run
selects the run; without it the latest run is shown. --format json|yaml emits
the machine-readable view.

Examples:
  sdt research inspect
  sdt research inspect --run 01a0f…  --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, _, err := resolveRun(cmd)
		exitWithError(cmd, err)

		counts := map[string]int{}
		for _, s := range r.Sources {
			counts[s.Status]++
		}
		view := researchInspectView{
			RunID:      r.RunID,
			Query:      r.Query,
			Scope:      r.Scope,
			Created:    r.Created,
			Updated:    r.Updated,
			Budget:     r.Budget,
			Checkpoint: r.Checkpoint,
			Sources:    r.Sources,
			Counts:     counts,
		}
		outputResearch(cmd, view, func() string { return researchInspectText(r, counts) })
	},
}

func researchInspectText(r *research.Run, counts map[string]int) string {
	out := fmt.Sprintf("run %s\n  query: %s\n", r.RunID, r.Query)
	if r.Scope != "" {
		out += fmt.Sprintf("  scope: %s\n", r.Scope)
	}
	out += fmt.Sprintf("  created: %s\n  updated: %s\n", r.Created, r.Updated)
	if r.Checkpoint.Operation != "" {
		out += fmt.Sprintf("  checkpoint: %s (%s)\n", r.Checkpoint.Operation, r.Checkpoint.At)
	}
	if r.Budget.MaxResults > 0 || r.Budget.MaxCredits > 0 {
		out += fmt.Sprintf("  budget: results<=%d credits<=%d used(credits=%d results=%d)\n",
			r.Budget.MaxResults, r.Budget.MaxCredits, r.Budget.CreditsUsed, r.Budget.Results)
	}
	if len(r.Sources) == 0 {
		out += "  sources: none\n"
		return out
	}
	out += fmt.Sprintf("  sources: %d\n", len(r.Sources))
	for _, s := range r.Sources {
		line := fmt.Sprintf("    - [%s] %s", s.Status, s.CanonicalURL)
		if s.Title != "" {
			line += " — " + s.Title
		}
		if s.Error != "" {
			line += " (" + s.Error + ")"
		}
		out += line + "\n"
	}
	if len(counts) > 0 {
		out += "  counts:"
		for _, st := range []string{research.StatusDiscovered, research.StatusFetched, research.StatusParsed, research.StatusVerified, research.StatusRejected} {
			if n := counts[st]; n > 0 {
				out += fmt.Sprintf(" %s=%d", st, n)
			}
		}
		out += "\n"
	}
	return out
}

func init() {
	researchInitCmd.Flags().String("query", "", "The question the run answers")
	researchInitCmd.Flags().String("scope", "", "Optional scope narrowing the run")
	researchInspectCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchInitCmd, researchInspectCmd)
	rootCmd.AddCommand(researchCmd)
}
