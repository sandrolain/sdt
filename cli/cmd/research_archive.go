package cmd

import (
	"fmt"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// archiveVerified copies every verified source's raw capture into the run's
// dated, objective-scoped directory under context/refs/, returning the refs path
// per canonical URL. It is the single archive implementation, shared by
// `research archive` and (until its reduction) `research populate-wiki`.
func archiveVerified(root string, r *research.Run) (map[string]string, error) {
	refs := map[string]string{}
	for _, s := range r.Sources {
		if s.Status != research.StatusVerified {
			continue
		}
		rel, err := r.ArchiveSource(root, s)
		if err != nil {
			return refs, err
		}
		refs[s.CanonicalURL] = rel
	}
	return refs, nil
}

var researchArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive a run's verified captures into context/refs/ (gated)",
	Long: `Archive the verified captures of a research run into the run's dated,
objective-scoped directory under context/refs/. The default is a PREVIEW that
writes nothing.

--apply performs the archive, but only when you also pass --confirm: it copies
each verified capture, records the run's archive directory and prints the
citation pack (refs/<dir>/<result>.md@<sha8>) the wiki page cites. A source SDT
could not verify is never archived; refused sources are listed with the reason.
No wiki page is written: populating the wiki is an agent editorial task.

Examples:
  sdt research archive --run 01a0f…
  sdt research archive --run 01a0f… --format json
  sdt research archive --run 01a0f… --apply --confirm`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		plan := r.PlanArchive()
		if !getBoolFlag(cmd, "apply", false) {
			outputResearch(cmd, plan, func() string { return plan.RenderPlan() })
			return
		}
		if !getBoolFlag(cmd, "confirm", false) {
			exitWithError(cmd, fmt.Errorf("--apply needs --confirm: the archive writes into context/refs/"))
			return
		}
		if !plan.Establishable() {
			exitWithError(cmd, fmt.Errorf("nothing to archive: no verified source"))
			return
		}

		if _, aerr := archiveVerified(root, r); aerr != nil {
			exitWithError(cmd, aerr)
			return
		}
		r.MarkCheckpoint("archive")
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputResearch(cmd, plan, func() string { return plan.RenderApplied() })
	},
}

func init() {
	researchArchiveCmd.Flags().Bool("apply", false, "Apply the archive (default: preview only)")
	researchArchiveCmd.Flags().Bool("confirm", false, "Confirm the write into context/refs/ (required with --apply)")
	researchArchiveCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchArchiveCmd)
}
