package cmd

import (
	"github.com/spf13/cobra"
)

var researchPopulateWikiCmd = &cobra.Command{
	Use:   "populate-wiki",
	Short: "List verified sources and citations for agent-authored wiki pages",
	Long: `List a run's verified sources with the citation each wiki page must carry
(refs/<dir>/<result>.md@<sha8>). This is a READ-ONLY brief: the CLI never writes
a context/wiki/ page. Populating the wiki is an editorial task — the agent reads
the captures, distils concepts per context/instructions/ingestion.md and writes
the pages.

Archive the captures first with ` + "`sdt research archive --apply --confirm`" + `.
` + "`--brief`" + ` records why the knowledge is useful to the project.

Examples:
  sdt research populate-wiki --run 01a0f…
  sdt research populate-wiki --run 01a0f… --brief "embedded vector stores" --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, _, err := resolveRun(cmd)
		exitWithError(cmd, err)

		brief := getStringFlag(cmd, "brief", false)
		view := r.PlanWikiBrief(brief)
		outputResearch(cmd, view, func() string { return view.Render() })
	},
}

func init() {
	researchPopulateWikiCmd.Flags().String("brief", "", "Why the knowledge is useful to the project")
	researchPopulateWikiCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchPopulateWikiCmd)
}
