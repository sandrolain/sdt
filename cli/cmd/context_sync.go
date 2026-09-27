package cmd

import (
	"encoding/json"
	"sort"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// contextSyncCmd is the standalone whole-store reconciler: it reuses the
// cascade derivations to repair documents whose declared status drifted from
// what their children imply, applying the flips (or only reporting them with
// --dry-run).

// syncStore reconciles every derived document (tasks, then plans, then
// analyses) so parents read their already-updated children. With dryRun it
// reports the would-be flips without writing.
func syncStore(dryRun bool) ([]cascadeChange, error) {
	store, err := loadCascadeStore()
	if err != nil {
		return nil, err
	}
	changes := make([]cascadeChange, 0)
	apply := func(n *cascadeNode, derived string) error {
		changes = append(changes, cascadeChange{Path: n.path, Kind: n.kind, From: n.status, To: derived})
		if dryRun {
			n.status = derived
			return nil
		}
		return applyDerivedStatus(n, derived)
	}
	for _, n := range sortedNodes(store.tasks) {
		if !statusFlappable(n) {
			continue
		}
		if d := deriveTaskStatus(n); d != "" && d != n.status && advances(n.kind, n.status, d) {
			if err := apply(n, d); err != nil {
				return changes, err
			}
		}
	}
	for _, n := range sortedNodes(store.plans) {
		if !statusFlappable(n) {
			continue
		}
		if d := store.derivePlanStatus(n); d != "" && d != n.status && advances(n.kind, n.status, d) {
			if err := apply(n, d); err != nil {
				return changes, err
			}
		}
	}
	for _, n := range sortedNodes(store.analyses) {
		if !statusFlappable(n) {
			continue
		}
		if d := store.deriveAnalysisStatus(n); d != "" && d != n.status && advances(n.kind, n.status, d) {
			if err := apply(n, d); err != nil {
				return changes, err
			}
		}
	}
	return changes, nil
}

func sortedNodes(m map[string]*cascadeNode) []*cascadeNode {
	out := make([]*cascadeNode, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func outputSyncResult(cmd *cobra.Command, changes []cascadeChange) {
	if changes == nil {
		changes = []cascadeChange{}
	}
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(changes, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(changes)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		if len(changes) == 0 {
			outputString(cmd, "nothing to do\n")
			return
		}
		for _, c := range changes {
			outputString(cmd, c.Path+": "+c.From+" → "+c.To+"\n")
		}
	}
}

var contextSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Recompute derived document statuses across the store",
	Long: `Reconcile declared and derived state across context/: a task file from its
checklist, a plan from its sourcing task files (plus its own checklists), an
analysis from its plans. It is idempotent — a clean store reports "nothing to
do" — and never touches archived/draft/abandoned documents or a document with
no children. --dry-run reports the flips without writing; --doc scopes the pass
to one subtree (the document and its ancestors).

Examples:
  sdt context sync
  sdt context sync --dry-run
  sdt context sync --doc plan/20260927-211743-cli-only-...-plan`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun := getBoolFlag(cmd, "dry-run", false)
		if ref := getStringFlag(cmd, "doc", false); ref != "" {
			doc, err := resolveContextDocPath(ref)
			exitWithError(cmd, err)
			changes, err := cascadeUp(normalizeContextRef(doc.Path), !dryRun)
			exitWithError(cmd, err)
			outputSyncResult(cmd, changes)
			return
		}
		changes, err := syncStore(dryRun)
		exitWithError(cmd, err)
		outputSyncResult(cmd, changes)
	},
}

func init() {
	contextSyncCmd.Flags().Bool("dry-run", false, "Report without writing anything")
	contextSyncCmd.Flags().String("doc", "", "Reconcile only this document and its ancestors")
}
