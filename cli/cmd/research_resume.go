package cmd

import (
	"fmt"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

var researchResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Continue an interrupted run without repeating finished work",
	Long: `Resume a run from its checkpoint: fetch the sources still 'discovered'
(never fetched, or a failed fetch to retry), then verify the sources that have a
payload. Work already done is skipped — a fetched or verified source is not
fetched again and not re-verified from scratch.

Replaying a completed run is idempotent: nothing new is fetched and no duplicate
source is created. The command exits non-zero when a resumed fetch fails.

Examples:
  sdt research resume
  sdt research resume --run 01a0f…`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		pending := r.PendingFetch()
		fetched, failed := 0, 0
		if len(pending) > 0 {
			fetched, failed = fetchTargets(r, root, pending, getStringFlag(cmd, "user-agent", false), getIntFlag(cmd, "timeout", false))
		}

		// Verify whatever has a payload but is not yet verified.
		verifyResult := r.Verify(research.RunDir(root, r.RunID))
		if verifyResult.Checked > 0 {
			r.MarkCheckpoint("resume")
		}

		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}

		outputResearch(cmd, struct {
			RunID   string                `json:"run_id" yaml:"run_id"`
			Fetched int                   `json:"fetched" yaml:"fetched"`
			Failed  int                   `json:"failed" yaml:"failed"`
			Verify  research.VerifyResult `json:"verify" yaml:"verify"`
		}{RunID: r.RunID, Fetched: fetched, Failed: failed, Verify: verifyResult}, func() string {
			return fmt.Sprintf("run %s: fetched %d, failed %d, verified %d, rejected %d\n",
				r.RunID, fetched, failed, verifyResult.Verified, verifyResult.Rejected)
		})

		if failed > 0 {
			exitWithError(cmd, fmt.Errorf("%d resumed fetch(es) failed", failed))
		}
		if verifyResult.Failed() {
			exitWithError(cmd, fmt.Errorf("%d source(s) rejected on resume", verifyResult.Rejected))
		}
	},
}

func init() {
	researchResumeCmd.Flags().Int("timeout", 60, "Request timeout in seconds")
	researchResumeCmd.Flags().String("user-agent", "sdt/1.0", "HTTP user agent for requests")
	researchResumeCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchResumeCmd)
}
