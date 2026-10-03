package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

var researchSynthesizeCmd = &cobra.Command{
	Use:   "synthesize",
	Short: "Assemble a cited draft from the run's verified sources",
	Long: `Assemble a deterministic, cited Markdown draft from the run's verified
sources: an outline plus one excerpt per source with a hash-anchored citation.
No model and no network are involved.

Unverified sources are excluded unless --include-unverified, in which case they
appear marked UNVERIFIED. Sources without a payload and rejected ones are listed
under "Skipped". The draft is written only to the target you name — a transient
path under context/tmp/ or a draft document; it never promotes anything into
approved knowledge.

Examples:
  sdt research synthesize --out context/tmp/draft.md
  sdt research synthesize --include-unverified --max-lines 40 --out draft.md`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		includeUnverified := getBoolFlag(cmd, "include-unverified", false)
		maxLines := getIntFlag(cmd, "max-lines", false)
		out := getStringFlag(cmd, "out", false)

		draft := r.Synthesize(research.RunDir(root, r.RunID), includeUnverified, maxLines)
		markdown := draft.Render()

		if out == "" {
			outputString(cmd, markdown)
			return
		}

		if !isTransientTarget(out) {
			exitWithError(cmd, fmt.Errorf("--out must be a transient path under context/tmp/ or a draft document, got %q", out))
			return
		}
		if err := writeDraft(out, markdown); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, fmt.Sprintf("wrote %s (%d cited source(s))\n", out, len(draft.Excerpts)))
	},
}

// isTransientTarget guards the promotion boundary: a synthesized draft may be
// written only under context/tmp/ or to a file the caller marks as a draft
// (a *.draft.md name). Anything else is refused.
func isTransientTarget(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "context/tmp/") || strings.HasPrefix(clean, "./context/tmp/") {
		return true
	}
	return strings.HasSuffix(clean, ".draft.md")
}

func writeDraft(path, markdown string) error {
	//#nosec G306 -- a user-named draft file, not a corpus document
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return err
	}
	return nil
}

func init() {
	researchSynthesizeCmd.Flags().String("out", "", "Write the draft here (default: stdout)")
	researchSynthesizeCmd.Flags().Bool("include-unverified", false, "Include unverified sources, marked as such")
	researchSynthesizeCmd.Flags().Int("max-lines", 0, "Cap the excerpt per source (0 = all)")
	researchSynthesizeCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchSynthesizeCmd)
}
