package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// learnStatusReport is the learning picture: who, how many goals, how much of the
// wiki is recorded, and what is weak or due.
type learnStatusReport struct {
	Learner  string      `json:"learner,omitempty" yaml:"learner,omitempty"`
	Since    string      `json:"since,omitempty" yaml:"since,omitempty"`
	Goals    int         `json:"goals" yaml:"goals"`
	Concepts int         `json:"concepts" yaml:"concepts"`
	Pages    int         `json:"pages" yaml:"pages"`
	Weak     []learnView `json:"weak,omitempty" yaml:"weak,omitempty"`
	Due      []learnView `json:"due,omitempty" yaml:"due,omitempty"`
}

var learnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report goals, coverage, weak areas and due reviews (read-only)",
	Long: `Report how far the learner has got against the wiki.

WHEN — you need the whole learning picture: goals, how many concepts are recorded,
what is weak and what is due.
WHEN NOT — you want to resolve a topic: use sdt learn inspect.

RETURNS — learner, goal count, recorded-concept count against the wiki page count,
plus the weak and due concepts. An absent learner state reports zeros rather than
failing. Reads only.

EXAMPLE — sdt learn status
          sdt learn status --format yaml`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		st := loadLearnState(cmd)
		bld := loadLearnWiki(cmd)
		report := learnStatusReport{
			Learner:  st.Profile.Name,
			Since:    st.Profile.Since,
			Goals:    len(st.Goals),
			Concepts: len(st.Concepts),
			Pages:    len(bld.Pages),
			Weak:     learnWeak(st),
		}
		report.Due, _ = learnReviewQueues(st, contextNow())
		learnEmit(cmd, report, func() {
			outputString(cmd, fmt.Sprintf("learner: %s\n", learnLearnerLine(learnResumeReport{
				Learner: report.Learner, Since: report.Since,
			})))
			outputString(cmd, fmt.Sprintf("goals: %d\n", report.Goals))
			outputString(cmd, fmt.Sprintf("concepts: %d recorded of %d wiki page(s)\n",
				report.Concepts, report.Pages))
			outputString(cmd, learnQueueLines("weak", report.Weak))
			outputString(cmd, learnQueueLines("due", report.Due))
		})
	},
}

func init() {
	learnCmd.AddCommand(learnStatusCmd)
}
