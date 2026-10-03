package cmd

import (
	"errors"
	"fmt"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

var researchVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check a run's recorded facts and mark sources verified/rejected",
	Long: `Verify the recorded facts of a run, offline and deterministically: it
recomputes the SHA-256 from each source's raw payload, checks the payload exists
and is non-empty, validates the manifest schema and marks each source
'verified' or 'rejected' with a reason. It never promotes a source it cannot
check.

Sources still 'discovered' (never fetched) are left untouched. The command exits
non-zero when any checked source is rejected — recorded, not enforcing.

Examples:
  sdt research verify
  sdt research verify --run 01a0f…  --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		res := r.Verify(research.RunDir(root, r.RunID))
		r.MarkCheckpoint("verify")
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}

		outputResearch(cmd, res, func() string { return verifyText(res) })
		if res.Checked == 0 {
			exitWithError(cmd, errors.New("nothing to verify: fetch some sources first"))
			return
		}
		if res.Failed() {
			exitWithError(cmd, fmt.Errorf("%d of %d checked source(s) rejected", res.Rejected, res.Checked))
		}
	},
}

func verifyText(res research.VerifyResult) string {
	out := fmt.Sprintf("run %s: checked %d, verified %d, rejected %d\n", res.RunID, res.Checked, res.Verified, res.Rejected)
	for _, o := range res.Outcomes {
		line := fmt.Sprintf("  - [%s] %s", o.Status, o.CanonicalURL)
		if o.Reason != "" {
			line += " — " + o.Reason
		}
		out += line + "\n"
	}
	return out
}

func init() {
	researchVerifyCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchVerifyCmd)
}
