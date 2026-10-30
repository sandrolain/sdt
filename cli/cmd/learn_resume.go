package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// learnAction is one ranked next action with the reason it was chosen. The list is
// capped, so a session starts with a decision rather than a queue.
type learnAction struct {
	Concept string `json:"concept" yaml:"concept"`
	Action  string `json:"action" yaml:"action"` // review | revisit | prerequisite | start
	Why     string `json:"why" yaml:"why"`
}

// learnMaxActions is how many next actions a resume reports. Two is a decision;
// more is the review queue, which `learn review` already prints.
const learnMaxActions = 2

// learnResumeReport is where a learning session stands: who the learner is, the
// last recorded session, the goals, and what is due now.
type learnResumeReport struct {
	Learner     string        `json:"learner,omitempty" yaml:"learner,omitempty"`
	Since       string        `json:"since,omitempty" yaml:"since,omitempty"`
	LastSession string        `json:"last_session,omitempty" yaml:"last_session,omitempty"`
	Goals       int           `json:"goals" yaml:"goals"`
	Due         []learnView   `json:"due,omitempty" yaml:"due,omitempty"`
	Next        []learnView   `json:"upcoming,omitempty" yaml:"upcoming,omitempty"`
	Actions     []learnAction `json:"next_actions,omitempty" yaml:"next_actions,omitempty"`
}

var learnResumeCmd = &cobra.Command{
	Use:   verbResume,
	Short: "Resume the learning session: profile, last session and the review queue (read-only)",
	Long: `Report the learner's profile, the last recorded session and what is due.

WHEN — a session resumes and the agent needs the standing before teaching.
WHEN NOT — you want the full picture (goals, coverage, weak areas): use
sdt learn status.

RETURNS — learner, last session, goal count, the review queue split into due and
upcoming, and at most two ranked next actions, each with the reason it was chosen
(due > weak > blocked prerequisite > new). An absent learner state reports an empty
session rather than failing. Reads only.

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
		report.Actions = learnNextActions(st, loadLearnWiki(cmd), now)
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
			outputString(cmd, learnActionLines(report.Actions))
		})
	},
}

// learnNextActions ranks what to do next: due first, then weak, then a blocked
// prerequisite, then a new page whose prerequisites are already solid. The order is
// fixed, a concept appears once, and the list is capped.
func learnNextActions(st *learn.State, bld *contextwiki.Builder, now time.Time) []learnAction {
	var out []learnAction
	seen := map[string]bool{}
	add := func(concept, action, why string) {
		if seen[concept] || len(out) >= learnMaxActions {
			return
		}
		seen[concept] = true
		out = append(out, learnAction{Concept: concept, Action: action, Why: why})
	}
	due, _ := learnReviewQueues(st, now)
	for _, v := range due {
		add(v.ID, "review", "due "+v.NextReview)
	}
	for _, v := range learnWeak(st) {
		if learnDue(v.NextReview, now) {
			continue
		}
		why := "standing is " + learnStatusWord(v)
		if strings.TrimSpace(v.Misconception) != "" {
			why = "open misconception"
		}
		add(v.ID, "revisit", why)
	}
	for i := range st.Concepts {
		c := &st.Concepts[i]
		if assessmentUnderstanding(c) == learn.UnderstandingSolid {
			continue
		}
		for _, prereq := range c.Prerequisites {
			if p := learnConceptByID(st, prereq); p != nil &&
				assessmentUnderstanding(p) == learn.UnderstandingSolid {
				continue
			}
			add(prereq, "prerequisite", "blocks "+c.ID)
			break
		}
	}
	for _, p := range bld.Pages {
		if learnRecord(st, p.ID) != nil {
			continue
		}
		if !learnPrerequisitesSolid(st, bld, p) {
			continue
		}
		add(learn.ConceptID(p.ID), "start", "not yet recorded")
	}
	return out
}

// learnConceptByID returns the state record with a stable id, or nil.
func learnConceptByID(st *learn.State, id string) *learn.Concept {
	for i := range st.Concepts {
		if st.Concepts[i].ID == id {
			return &st.Concepts[i]
		}
	}
	return nil
}

// learnPrerequisitesSolid reports whether every prerequisite of a page is recorded
// solid. A page with no prerequisite is reachable.
func learnPrerequisitesSolid(st *learn.State, bld *contextwiki.Builder, p *contextwiki.Page) bool {
	for _, id := range learnPrerequisites(bld, p) {
		rec := learnConceptByID(st, learn.ConceptID(id))
		if rec == nil || assessmentUnderstanding(rec) != learn.UnderstandingSolid {
			return false
		}
	}
	return true
}

// learnActionLines renders the ranked next actions.
func learnActionLines(actions []learnAction) string {
	if len(actions) == 0 {
		return "next: nothing queued\n"
	}
	var b strings.Builder
	b.WriteString("next:\n")
	for _, a := range actions {
		fmt.Fprintf(&b, "  %-12s %-26s %s\n", a.Action, a.Concept, a.Why)
	}
	return b.String()
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
