package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn answer (deterministic grading) ──────────────────────────────────
//
// Grading is read-only: it reads the item's frozen key from the lesson and returns
// a verdict. It never writes learner state — recording the verdict is a separate,
// explicit `sdt learn record`.

// learnAnswerReport is the graded verdict of one answer.
type learnAnswerReport struct {
	Concept  string  `json:"concept" yaml:"concept"`
	Item     string  `json:"item" yaml:"item"`
	Type     string  `json:"type" yaml:"type"`
	Grade    string  `json:"grade" yaml:"grade"`
	Rule     string  `json:"rule" yaml:"rule"`
	Expected string  `json:"expected" yaml:"expected"`
	Rubric   string  `json:"rubric,omitempty" yaml:"rubric,omitempty"`
	Coverage float64 `json:"coverage,omitempty" yaml:"coverage,omitempty"`
}

var learnAnswerCmd = &cobra.Command{
	Use:   "answer --concept <concept> --item <item> --answer <text>",
	Short: "Grade one answer against an item's frozen key (read-only)",
	Long: `Grade one answer deterministically and say which rule decided it.

WHEN — an answer has been given and must be checked before anything is recorded.
WHEN NOT — you want the questions: use sdt learn quiz. You want the key in bulk:
sdt learn quiz --with-key.

RETURNS — the grade (` + "`correct`" + `, ` + "`partially_correct`" + `, ` + "`incorrect`" + ` or
` + "`ungradable`" + `), the rule that produced it (` + "`exact`" + `, ` + "`numeric`" + `,
` + "`keywords`" + `, ` + "`manual`" + ` or the ` + "`auto`" + ` rule the item resolved to), the frozen key
and the rubric. Exit 0 for a correct or partially correct answer, 1 for an
incorrect one, 4 when the answer cannot be graded or the concept or item is not in
the corpus. Reads only; ` + "`sdt learn record`" + ` is what writes the verdict.

EXAMPLE — sdt learn answer --concept architecture/four-key-metrics --item fkm-3 --answer "5.56%"
          sdt learn answer --concept architecture/four-key-metrics --item fkm-1 --answer "deployment frequency" --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		concept := strings.TrimSpace(getStringFlag(cmd, "concept", false))
		itemID := strings.TrimSpace(getStringFlag(cmd, "item", false))
		answer := getStringFlag(cmd, "answer", false)
		if concept == "" || itemID == "" || strings.TrimSpace(answer) == "" {
			exitWithError(cmd, fmt.Errorf("--concept, --item and --answer are all required"))
			return
		}
		bld := loadLearnWiki(cmd)
		page := learnResolveConcept(bld, concept)
		if page == nil {
			learnExitWith(fmt.Errorf("no wiki page matches the concept %q: a grade anchors to a corpus item", concept), 4)
			return
		}
		item, ok := learnLessonItemByID(page.ID, itemID)
		if !ok {
			learnExitWith(fmt.Errorf("concept %q has no item %q in its lesson", page.ID, itemID), 4)
			return
		}
		verdict := learn.GradeAnswer(item.Grading, item.Expected, answer, item.Type)
		report := learnAnswerReport{
			Concept: page.ID, Item: item.ID, Type: item.Type,
			Grade: verdict.Grade, Rule: verdict.Rule,
			Expected: item.Expected, Rubric: item.Rubric, Coverage: verdict.Coverage,
		}
		learnEmit(cmd, report, func() {
			line := fmt.Sprintf("%s %s: %s (rule %s)", page.ID, item.ID, verdict.Grade, verdict.Rule)
			if verdict.Coverage > 0 {
				line += fmt.Sprintf(", coverage %.0f%%", verdict.Coverage*100)
			}
			outputString(cmd, line+"\n")
			outputString(cmd, "  expected: "+item.Expected+"\n")
			if item.Rubric != "" {
				outputString(cmd, "  rubric: "+item.Rubric+"\n")
			}
		})
		switch verdict.Grade {
		case learn.GradeIncorrect:
			learnExitWith(nil, 1)
		case learn.GradeUngradable:
			learnExitWith(nil, 4)
		}
	},
}

// learnExitWith reports a message through the shared logger and exits with the
// given code. Code 1 goes through exitWithError so every command reports the same
// way; 4 means "cannot grade" or "not in the corpus", a verdict rather than a
// failure of the tool.
func learnExitWith(err error, code int) {
	if err != nil {
		slog.Error(err.Error())
	}
	exit(code)
}

func init() {
	f := learnAnswerCmd.Flags()
	f.String("concept", "", "Wiki page id (or title) of the concept")
	f.String("item", "", "Item id from the lesson's `## Assess` table")
	f.String("answer", "", "The answer to grade")
	learnCmd.AddCommand(learnAnswerCmd)
}
