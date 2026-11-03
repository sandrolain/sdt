package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/learn"
)

// ── the closing next step ─────────────────────────────────────────────────────
//
// Every read-only verb ends its text reply with one next verb, computed from the
// state it just reported. A learner reading a queue should not have to work out
// what to run next, and an agent should not have to guess. Text mode only: a JSON
// caller gets the data, never advice, and its shapes stay byte-identical.

// learnNextVerb ranks the single next command for a state: an empty item bank
// first (nothing can be quizzed), then the review queue, then the quiz of the
// concept just read, and `resume` when the reply was not about one concept.
func learnNextVerb(st *learn.State, concept string, now time.Time) string {
	if concept != "" {
		if items, ok := learnAssessmentItems(concept); !ok || len(items) == 0 {
			return "sdt learn inspect " + concept
		}
	}
	if due, _ := learnReviewQueues(st, now); len(due) > 0 {
		return "sdt learn review --limit 5"
	}
	if weak := learnWeak(st); len(weak) > 0 {
		return "sdt learn review"
	}
	if concept != "" {
		return "sdt learn quiz " + concept
	}
	return "sdt learn resume"
}

// learnSuggestNext prints the closing line, in text mode only.
func learnSuggestNext(cmd *cobra.Command, st *learn.State, concept string) {
	if getFormat(cmd) != formatText {
		return
	}
	if next := learnNextVerb(st, concept, contextNow()); next != "" {
		outputString(cmd, "📍 next: "+next+"\n")
	}
}
