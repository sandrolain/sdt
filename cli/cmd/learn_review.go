package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/learn"
)

// learnReviewEntry is one row of the review queue: a concept, why it is queued
// (due or weak) and how far its review date sits from today.
type learnReviewEntry struct {
	Concept learnView `json:"concept" yaml:"concept"`
	Reason  string    `json:"reason" yaml:"reason"` // due | weak
	DueIn   int       `json:"due_in_days" yaml:"due_in_days"`
}

// learnReviewQueues splits the recorded concepts into the due ones (review date
// reached) and the upcoming ones, each ordered by review date then id.
func learnReviewQueues(st *learn.State, now time.Time) (due, upcoming []learnView) {
	for i := range st.Concepts {
		c := &st.Concepts[i]
		if c.NextReview == "" {
			continue
		}
		view := learnViewOf(c)
		if learnDue(c.NextReview, now) {
			due = append(due, view)
		} else {
			upcoming = append(upcoming, view)
		}
	}
	byReview := func(xs []learnView) {
		sort.Slice(xs, func(i, j int) bool {
			if xs[i].NextReview != xs[j].NextReview {
				return xs[i].NextReview < xs[j].NextReview
			}
			return xs[i].ID < xs[j].ID
		})
	}
	byReview(due)
	byReview(upcoming)
	return due, upcoming
}

// assessmentUnderstanding returns a record's understanding level, empty when the
// concept was never assessed.
func assessmentUnderstanding(c *learn.Concept) string {
	if c.LastAssessed == nil {
		return ""
	}
	return c.LastAssessed.Understanding
}

// learnWeak returns the recorded concepts that are not yet solid, or that carry an
// open misconception, ordered by id.
func learnWeak(st *learn.State) []learnView {
	var out []learnView
	for i := range st.Concepts {
		c := &st.Concepts[i]
		level := assessmentUnderstanding(c)
		weak := level == learn.UnderstandingWeak || level == learn.UnderstandingPartial ||
			level == learn.UnderstandingUnknown || strings.TrimSpace(c.Misconception) != ""
		if !weak {
			continue
		}
		out = append(out, learnViewOf(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

var learnReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List the due and weak concepts as a bounded review queue (read-only)",
	Long: `Rank the recorded concepts by what needs revisiting.

WHEN — a review session starts, or you need the next concepts to revisit.
WHEN NOT — you want goals and coverage: use sdt learn status.

RETURNS — one row per queued concept: due first (review date reached), then weak
(concept not yet solid or carrying a misconception), each ordered by review date.
--limit bounds the queue (0 = all). Deterministic; reads only.

EXAMPLE — sdt learn review
          sdt learn review --limit 5 --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		st := loadLearnState(cmd)
		now := contextNow()
		due, _ := learnReviewQueues(st, now)
		rows := make([]learnReviewEntry, 0, len(due)+len(st.Concepts))
		for _, v := range due {
			rows = append(rows, learnReviewEntry{Concept: v, Reason: "due", DueIn: learnDueIn(v.NextReview, now)})
		}
		for _, v := range learnWeak(st) {
			if learnDue(v.NextReview, now) {
				continue // already queued as due
			}
			rows = append(rows, learnReviewEntry{Concept: v, Reason: "weak", DueIn: learnDueIn(v.NextReview, now)})
		}
		if limit := getIntFlag(cmd, "limit", false); limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		learnEmit(cmd, rows, func() {
			if len(rows) == 0 {
				outputString(cmd, "nothing due and nothing weak\n")
				return
			}
			for _, r := range rows {
				line := fmt.Sprintf("%-5s %-26s %s", r.Reason, r.Concept.ID, r.Concept.Title)
				if r.Concept.Understanding != "" {
					line += " (" + r.Concept.Understanding + ")"
				}
				outputString(cmd, line+"\n")
			}
		})
	},
}

// learnDueIn returns the whole days between today and a review date (negative
// when overdue, zero when unparseable or unscheduled).
func learnDueIn(nextReview string, now time.Time) int {
	if nextReview == "" {
		return 0
	}
	due, err := time.Parse(learn.DateLayout, nextReview)
	if err != nil {
		return 0
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(due.Sub(today).Hours() / 24)
}

func init() {
	learnReviewCmd.Flags().Int("limit", 0, "Maximum rows to queue (0 = all)")
	learnCmd.AddCommand(learnReviewCmd)
}
