package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// learnInspectEntry is one resolved concept with the prerequisites to teach first.
type learnInspectEntry struct {
	Concept       learnView `json:"concept" yaml:"concept"`
	Prerequisites []string  `json:"prerequisites,omitempty" yaml:"prerequisites,omitempty"`
}

var learnInspectCmd = &cobra.Command{
	Use:   "inspect [topic]",
	Short: "Resolve a topic to wiki concepts and their prerequisites (read-only)",
	Long: `Resolve a topic against the wiki graph and report what covers it.

WHEN — you need the concepts behind a topic, what they depend on, and which the
learner has already recorded.
WHEN NOT — the corpus has no matching page: this reports what the corpus has and
stops; it never answers from recall.

RETURNS — one entry per matching page with its prerequisite ids and the learner's
recorded standing, ordered by page id. Reads only. --limit bounds the report
(0 = all).

EXAMPLE — sdt learn inspect "resolution order"
          sdt learn inspect backend/auth --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		topic := strings.Join(args, " ")
		if strings.TrimSpace(topic) == "" {
			exitWithError(cmd, fmt.Errorf("a topic is required: sdt learn inspect <topic>"))
			return
		}
		st := loadLearnState(cmd)
		bld := loadLearnWiki(cmd)
		pages := learnPagesFor(bld, topic)
		if limit := getIntFlag(cmd, "limit", false); limit > 0 && len(pages) > limit {
			pages = pages[:limit]
		}
		out := make([]learnInspectEntry, 0, len(pages))
		for _, p := range pages {
			out = append(out, learnInspectEntry{
				Concept:       learnViewFor(p, st),
				Prerequisites: learnPrerequisites(bld, p),
			})
		}
		learnEmit(cmd, out, func() {
			if len(out) == 0 {
				outputString(cmd, "no wiki page matches the topic\n")
				return
			}
			for _, e := range out {
				outputString(cmd, fmt.Sprintf("%-9s %-26s %s\n",
					learnStatusWord(e.Concept), e.Concept.ID, e.Concept.Title))
				if len(e.Prerequisites) > 0 {
					outputString(cmd, "          prerequisites: "+strings.Join(e.Prerequisites, ", ")+"\n")
				}
			}
		})
	},
}

func init() {
	learnInspectCmd.Flags().Int("limit", 0, "Maximum concepts to report (0 = all)")
	learnCmd.AddCommand(learnInspectCmd)
}
