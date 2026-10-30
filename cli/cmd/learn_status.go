package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// learnCoverageCounts separates how much of the corpus was touched from how well it
// is known. Coverage is a fact about exposure; mastery is a fact about
// demonstrated understanding. They are reported as two lines and never folded into
// one number.
type learnCoverageCounts struct {
	Unseen   int `json:"unseen" yaml:"unseen"`
	Covered  int `json:"covered" yaml:"covered"`
	Assessed int `json:"assessed" yaml:"assessed"`
}

// learnMasteryCounts counts the recorded concepts by standing.
type learnMasteryCounts struct {
	Solid   int `json:"solid" yaml:"solid"`
	Partial int `json:"partial" yaml:"partial"`
	Weak    int `json:"weak" yaml:"weak"`
	Unknown int `json:"unknown" yaml:"unknown"`
}

// learnStatusReport is the learning picture: who, how many goals, how much of the
// wiki is recorded, and what is weak or due.
type learnStatusReport struct {
	Learner  string              `json:"learner,omitempty" yaml:"learner,omitempty"`
	Since    string              `json:"since,omitempty" yaml:"since,omitempty"`
	Goals    int                 `json:"goals" yaml:"goals"`
	Concepts int                 `json:"concepts" yaml:"concepts"`
	Pages    int                 `json:"pages" yaml:"pages"`
	Coverage learnCoverageCounts `json:"coverage" yaml:"coverage"`
	Mastery  learnMasteryCounts  `json:"mastery" yaml:"mastery"`
	Weak     []learnView         `json:"weak,omitempty" yaml:"weak,omitempty"`
	Due      []learnView         `json:"due,omitempty" yaml:"due,omitempty"`
}

var learnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report goals, coverage, weak areas and due reviews (read-only)",
	Long: `Report how far the learner has got against the wiki.

WHEN — you need the whole learning picture: goals, how many concepts are recorded,
what is weak and what is due.
WHEN NOT — you want to resolve a topic: use sdt learn inspect.

RETURNS — learner, goal count, recorded-concept count against the wiki page count,
coverage and mastery as two separate lines, plus the weak and due concepts. Coverage
is exposure (` + "`assessed`" + `, ` + "`covered`" + `, ` + "`unseen`" + `); mastery is the standing of
the recorded concepts (` + "`solid`" + `, ` + "`partial`" + `, ` + "`weak`" + `, ` + "`unknown`" + `), and the two
are never folded into one number. An absent learner state reports zeros rather than
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
			Coverage: learnCoverageOf(bld, st),
			Mastery:  learnMasteryOf(st),
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
			outputString(cmd, fmt.Sprintf("coverage: %d assessed, %d covered, %d unseen of %d page(s)\n",
				report.Coverage.Assessed, report.Coverage.Covered, report.Coverage.Unseen, report.Pages))
			outputString(cmd, fmt.Sprintf("mastery: %d solid, %d partial, %d weak, %d unknown\n",
				report.Mastery.Solid, report.Mastery.Partial, report.Mastery.Weak, report.Mastery.Unknown))
			outputString(cmd, learnQueueLines("weak", report.Weak))
			outputString(cmd, learnQueueLines("due", report.Due))
		})
	},
}

// learnCoverageOf counts the wiki pages by how far the learner got: unseen (no
// record), covered (recorded but never assessed) and assessed (a rating exists).
func learnCoverageOf(bld *contextwiki.Builder, st *learn.State) learnCoverageCounts {
	var c learnCoverageCounts
	for _, p := range bld.Pages {
		rec := learnRecord(st, p.ID)
		switch {
		case rec == nil:
			c.Unseen++
		case rec.LastAssessed == nil:
			c.Covered++
		default:
			c.Assessed++
		}
	}
	return c
}

// learnMasteryOf counts the recorded concepts by standing. An unassessed record
// counts as unknown, never as solid.
func learnMasteryOf(st *learn.State) learnMasteryCounts {
	var m learnMasteryCounts
	for i := range st.Concepts {
		switch assessmentUnderstanding(&st.Concepts[i]) {
		case learn.UnderstandingSolid:
			m.Solid++
		case learn.UnderstandingPartial:
			m.Partial++
		case learn.UnderstandingWeak:
			m.Weak++
		default:
			m.Unknown++
		}
	}
	return m
}

func init() {
	learnCmd.AddCommand(learnStatusCmd)
}
