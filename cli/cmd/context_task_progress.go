package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxTaskPhaseLabelRegexp captures the label of a `## Phase <label>` heading in
// a task file, so `task progress` can walk the phases in order.

var ctxTaskPhaseLabelRegexp = regexp.MustCompile(`(?i)^##\s+phase\s+([0-9a-z]+)`)

// taskPhaseProgress is one phase's progress: items done over total, the blocked
// ids, and whether it is the current (first unfinished) phase.

type taskPhaseProgress struct {
	Phase   string   `json:"phase" yaml:"phase"`
	Done    int      `json:"done" yaml:"done"`
	Total   int      `json:"total" yaml:"total"`
	Blocked []string `json:"blocked,omitempty" yaml:"blocked,omitempty"`
	Current bool     `json:"current" yaml:"current"`
}

// taskPhaseProgresses computes the per-phase progress of a task file, in heading
// order; the first phase with an unfinished item is marked current (F29).

func taskPhaseProgresses(content string) []taskPhaseProgress {
	var labels []string
	for _, line := range strings.Split(content, "\n") {
		if m := ctxTaskPhaseLabelRegexp.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			labels = append(labels, m[1])
		}
	}
	out := make([]taskPhaseProgress, 0, len(labels))
	for _, label := range labels {
		items := taskItemsInSection(content, label)
		p := taskPhaseProgress{Phase: label, Total: len(items)}
		for _, it := range items {
			switch it.Status {
			case taskStatusDone:
				p.Done++
			case taskStatusBlocked:
				p.Blocked = append(p.Blocked, it.ID)
			}
		}
		out = append(out, p)
	}
	for i := range out {
		if out[i].Done < out[i].Total {
			out[i].Current = true
			break
		}
	}
	return out
}

// contextTaskProgressCmd answers "where are we" without grepping the file (F29):
// it prints each phase's N/M and the blocked ids, and flags the current phase.

var contextTaskProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Show a task file's per-phase progress (N/M, current phase, blocked ids)",
	Long: `Show the per-phase progress of the active plan's task file: each phase's
done/total count, the blocked item ids, and which phase is current (the first
unfinished one). Read-only.

Examples:
  sdt context task progress
  sdt context task progress --plan plan/<plan-file>`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		plan := strings.TrimSpace(getStringFlag(cmd, "plan", false))
		if plan == "" {
			plan = latestActivePlan()
			if plan == "" {
				exitWithError(cmd, errors.New("no active plan found; pass --plan <plan-file>"))
				return
			}
		}
		content, err := readTaskFile("", taskStreamFlag(cmd), plan)
		exitWithError(cmd, err)
		phases := taskPhaseProgresses(content)
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(phases, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(phases)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if len(phases) == 0 {
				outputString(cmd, "no phases\n")
				return
			}
			var b strings.Builder
			for _, p := range phases {
				fmt.Fprintf(&b, "phase %s\t%d/%d done", p.Phase, p.Done, p.Total)
				if p.Current {
					b.WriteString("\t(current)")
				}
				if len(p.Blocked) > 0 {
					fmt.Fprintf(&b, "\tblocked: %s", strings.Join(p.Blocked, ", "))
				}
				b.WriteString("\n")
			}
			outputString(cmd, b.String())
		}
	},
}
