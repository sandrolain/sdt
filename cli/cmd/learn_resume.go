package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// learnResumeReport is where a learning session stands: who the learner is, the
// last recorded session, the goals, and what is due now.
type learnResumeReport struct {
	Learner     string      `json:"learner,omitempty" yaml:"learner,omitempty"`
	Since       string      `json:"since,omitempty" yaml:"since,omitempty"`
	LastSession string      `json:"last_session,omitempty" yaml:"last_session,omitempty"`
	Goals       int         `json:"goals" yaml:"goals"`
	Due         []learnView `json:"due,omitempty" yaml:"due,omitempty"`
	Next        []learnView `json:"upcoming,omitempty" yaml:"upcoming,omitempty"`
}

var learnResumeCmd = &cobra.Command{
	Use:   verbResume,
	Short: "Resume the learning session: profile, last session and the review queue (read-only)",
	Long: `Report the learner's profile, the last recorded session and what is due.

WHEN — a session resumes and the agent needs the standing before teaching.
WHEN NOT — you want the full picture (goals, coverage, weak areas): use
sdt learn status.

RETURNS — learner, last session, goal count and the review queue split into due
and upcoming. An absent learner state reports an empty session rather than
failing. Reads only.

EXAMPLE — sdt learn resume
          sdt learn resume --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		st := loadLearnState(cmd)
		now := contextNow()
		report := learnResumeReport{
			Learner: st.Profile.Name,
			Since:   st.Profile.Since,
			Goals:   len(st.Goals),
		}
		if n := len(st.Sessions); n > 0 {
			report.LastSession = st.Sessions[n-1].At
		}
		report.Due, report.Next = learnReviewQueues(st, now)
		learnEmit(cmd, report, func() {
			outputString(cmd, fmt.Sprintf("learner: %s\n", learnLearnerLine(report)))
			if report.LastSession == "" {
				outputString(cmd, "last session: none recorded\n")
			} else {
				outputString(cmd, "last session: "+report.LastSession+"\n")
			}
			outputString(cmd, fmt.Sprintf("goals: %d\n", report.Goals))
			outputString(cmd, learnQueueLines("due", report.Due))
			outputString(cmd, learnQueueLines("upcoming", report.Next))
		})
	},
}

// learnLearnerLine renders the learner identity, or the absence of one.
func learnLearnerLine(r learnResumeReport) string {
	if r.Learner == "" {
		return "not recorded (no learner state yet)"
	}
	if r.Since != "" {
		return r.Learner + " (since " + r.Since + ")"
	}
	return r.Learner
}

// learnQueueLines renders a named review queue.
func learnQueueLines(label string, views []learnView) string {
	if len(views) == 0 {
		return label + ": none\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", label)
	for _, v := range views {
		note := v.NextReview
		if v.Understanding != "" {
			note = strings.TrimSpace(v.Understanding + " " + note)
		}
		fmt.Fprintf(&b, "  %-26s %s\n", v.ID, strings.TrimSpace(note))
	}
	return b.String()
}

func init() {
	learnCmd.AddCommand(learnResumeCmd)
}
