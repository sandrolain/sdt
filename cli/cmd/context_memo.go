package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/memo"
)

// ── context memo (planned-operation register) ─────────────────────────────────
//
// context/memo.yaml is CLI-owned: these verbs are the only writer. The register
// holds recurring `rules` (a cadence) and one-off `memos` (a due date); `due`
// computes what is due from the system clock (advisory, never enforcement) and
// `done` records a claim after the operation succeeded.

var contextMemoCmd = &cobra.Command{
	Use:   "memo",
	Short: "Manage the planned-operation register (rules and memos)",
	Long: `Manage context/memo.yaml: recurring rules (a cadence) and one-off memos
(a due date), together with the last-run state.

  sdt context memo add rule --id <id> --summary <s> --every-days <n>
  sdt context memo add memo --id <id> --summary <s> --due <YYYY-MM-DD>
  sdt context memo list
  sdt context memo due [--today <YYYY-MM-DD>] [--check]
  sdt context memo done <id>
  sdt context memo remove <id>

` + "`due`" + ` prints nothing when nothing is due. It is advisory by default and
exits non-zero only with ` + "`--check`" + `, for scripts (see decision 0023).`,
}

var contextMemoAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a rule or a memo",
	Long:  "Add a recurring rule (`add rule`) or a one-off memo (`add memo`) to context/memo.yaml.",
}

var contextMemoAddRuleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Add a recurring rule",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		reg := loadMemoRegister(cmd)
		id := sanitizeSlug(getStringFlag(cmd, "id", true))
		if _, _, ok := reg.Find(id); ok {
			exitWithError(cmd, fmt.Errorf("an entry with id %q already exists", id))
			return
		}
		rule := memo.MakeRule(id, getStringFlag(cmd, "summary", true), getIntFlag(cmd, "every-days", true))
		rule.LastDone = getStringFlag(cmd, "last-done", false)
		rule.Action = getStringFlag(cmd, "action", false)
		rule.Source = getStringFlag(cmd, "source", false)
		if getBoolFlag(cmd, "disabled", false) {
			disabled := false
			rule.Enabled = &disabled
		}
		reg.Rules = append(reg.Rules, rule)
		saveMemoRegister(cmd, reg)
		outputMemoResult(cmd, "added rule", id)
	},
}

var contextMemoAddMemoCmd = &cobra.Command{
	Use:   "memo",
	Short: "Add a one-off memo",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		reg := loadMemoRegister(cmd)
		id := sanitizeSlug(getStringFlag(cmd, "id", true))
		if _, _, ok := reg.Find(id); ok {
			exitWithError(cmd, fmt.Errorf("an entry with id %q already exists", id))
			return
		}
		reg.Memos = append(reg.Memos, memo.Memo{
			ID:      id,
			Summary: getStringFlag(cmd, "summary", true),
			Due:     getStringFlag(cmd, "due", true),
			Action:  getStringFlag(cmd, "action", false),
			Source:  getStringFlag(cmd, "source", false),
		})
		saveMemoRegister(cmd, reg)
		outputMemoResult(cmd, "added memo", id)
	},
}

var contextMemoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the register entries",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		reg := loadMemoRegister(cmd)
		views := memoListViews(reg)
		if outputMemoStructured(cmd, views) {
			return
		}
		var b strings.Builder
		for _, v := range views {
			b.WriteString(memoListLine(v))
			b.WriteByte('\n')
		}
		outputString(cmd, b.String())
	},
}

var contextMemoDueCmd = &cobra.Command{
	Use:   "due",
	Short: "List the rules and memos due today",
	Long: `List the rules whose cadence has elapsed and the memos whose due date has
arrived, as of today (or --today). Text output is empty when nothing is due;
the exit status is 0 unless --check is given, which returns non-zero when
something is due (advisory otherwise — decision 0023).`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		today := memoToday(cmd)
		reg := loadMemoRegister(cmd)
		items, err := reg.DueOn(today)
		exitWithError(cmd, err)
		if outputMemoStructured(cmd, items) {
			if getBoolFlag(cmd, "check", false) && len(items) > 0 {
				exit(1)
			}
			return
		}
		var b strings.Builder
		for _, d := range items {
			b.WriteString(memoDueLine(d))
			b.WriteByte('\n')
		}
		outputString(cmd, b.String())
		if getBoolFlag(cmd, "check", false) && len(items) > 0 {
			exit(1)
		}
	},
}

var contextMemoDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Record a rule as done today, or a memo as done",
	Long: `Record that a rule was performed (sets last_done to today, or --today), or
that a memo is done. Call it only after the operation succeeded: it records a
claim, and SDT cannot verify the operation (decision 0023).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		reg := loadMemoRegister(cmd)
		kind, i, ok := reg.Find(args[0])
		if !ok {
			exitWithError(cmd, fmt.Errorf("no rule or memo with id %q", sanitizeSlug(args[0])))
			return
		}
		switch kind {
		case memo.KindRule:
			reg.Rules[i].LastDone = memoToday(cmd).Format(memo.DateLayout)
		case memo.KindMemo:
			reg.Memos[i].Done = true
		}
		saveMemoRegister(cmd, reg)
		outputMemoResult(cmd, "done", regRulesMemoID(reg, kind, i))
	},
}

var contextMemoRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a rule or a memo",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		reg := loadMemoRegister(cmd)
		id := sanitizeSlug(args[0])
		if !reg.Remove(id) {
			exitWithError(cmd, fmt.Errorf("no rule or memo with id %q", id))
			return
		}
		saveMemoRegister(cmd, reg)
		outputMemoResult(cmd, "removed", id)
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func loadMemoRegister(cmd *cobra.Command) *memo.Register {
	reg, err := memo.Load(".")
	if err != nil {
		exitWithError(cmd, err)
		return &memo.Register{}
	}
	return reg
}

func saveMemoRegister(cmd *cobra.Command, reg *memo.Register) {
	exitWithError(cmd, memo.Save(".", reg))
}

// memoToday resolves the reference day: the system UTC date, overridable with
// --today for deterministic tests and scripted runs.
func memoToday(cmd *cobra.Command) time.Time {
	s := getStringFlag(cmd, "today", false)
	if s == "" {
		return memo.Today()
	}
	t, err := time.Parse(memo.DateLayout, s)
	if err != nil {
		exitWithError(cmd, err)
		return memo.Today()
	}
	return memo.DateOnly(t)
}

// regRulesMemoID returns the id of the entry a mutation just addressed.
func regRulesMemoID(reg *memo.Register, kind string, i int) string {
	if kind == memo.KindRule {
		return reg.Rules[i].ID
	}
	return reg.Memos[i].ID
}

// memoListView is the serializable projection of a register entry for `list`.
type memoListView struct {
	Kind      string `json:"kind" yaml:"kind"`
	ID        string `json:"id" yaml:"id"`
	Summary   string `json:"summary" yaml:"summary"`
	EveryDays int    `json:"every_days,omitempty" yaml:"every_days,omitempty"`
	LastDone  string `json:"last_done,omitempty" yaml:"last_done,omitempty"`
	NextDue   string `json:"next_due,omitempty" yaml:"next_due,omitempty"`
	Due       string `json:"due,omitempty" yaml:"due,omitempty"`
	Done      bool   `json:"done,omitempty" yaml:"done,omitempty"`
	Enabled   bool   `json:"enabled" yaml:"enabled"`
}

func memoListViews(reg *memo.Register) []memoListView {
	views := []memoListView{}
	for _, r := range reg.Rules {
		v := memoListView{Kind: memo.KindRule, ID: r.ID, Summary: r.Summary, EveryDays: r.EveryDays, LastDone: r.LastDone, Enabled: r.EnabledNow()}
		if r.LastDone != "" {
			if last, err := time.Parse(memo.DateLayout, r.LastDone); err == nil {
				v.NextDue = last.AddDate(0, 0, r.EveryDays).Format(memo.DateLayout)
			}
		}
		views = append(views, v)
	}
	for _, m := range reg.Memos {
		views = append(views, memoListView{Kind: memo.KindMemo, ID: m.ID, Summary: m.Summary, Due: m.Due, Done: m.Done, Enabled: true})
	}
	return views
}

func memoListLine(v memoListView) string {
	if v.Kind == memo.KindRule {
		next := v.NextDue
		switch {
		case !v.Enabled:
			next = "disabled"
		case next == "":
			next = "due now"
		}
		return fmt.Sprintf("%s\t%s\tevery %dd\tnext %s", v.ID, v.Kind, v.EveryDays, next)
	}
	state := "due " + v.Due
	if v.Done {
		state = "done"
	}
	return fmt.Sprintf("%s\t%s\t%s", v.ID, v.Kind, state)
}

func memoDueLine(d memo.DueItem) string {
	switch {
	case d.DueDate == "":
		return fmt.Sprintf("%s\t%s\tdue (never done)", d.ID, d.Kind)
	case d.OverdueDays > 0:
		return fmt.Sprintf("%s\t%s\t%d day(s) overdue (due %s)", d.ID, d.Kind, d.OverdueDays, d.DueDate)
	default:
		return fmt.Sprintf("%s\t%s\tdue today (%s)", d.ID, d.Kind, d.DueDate)
	}
}

// memoResult is the serializable projection of a mutation for json/yaml output.
type memoResult struct {
	Action string `json:"action" yaml:"action"`
	ID     string `json:"id" yaml:"id"`
}

func outputMemoResult(cmd *cobra.Command, action, id string) {
	if outputMemoStructured(cmd, memoResult{Action: action, ID: id}) {
		return
	}
	outputString(cmd, action+" "+id+"\n")
}

// outputMemoStructured writes v as json/yaml for the selected --format and
// reports whether it handled the output (false for the default text format).
func outputMemoStructured(cmd *cobra.Command, v any) bool {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(v, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
		return true
	case fmtYAML:
		out, err := yaml.Marshal(v)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
		return true
	}
	return false
}

func init() {
	contextMemoAddRuleCmd.Flags().String("id", "", "Rule id (kebab-case slug, unique across rules and memos)")
	contextMemoAddRuleCmd.Flags().String("summary", "", "One-line description of the operation")
	contextMemoAddRuleCmd.Flags().Int("every-days", 0, "Cadence in days (positive integer)")
	contextMemoAddRuleCmd.Flags().String("last-done", "", "Day it was last done (YYYY-MM-DD; omit for due immediately)")
	contextMemoAddRuleCmd.Flags().String("action", "", "Optional project command or hint for the operation")
	contextMemoAddRuleCmd.Flags().String("source", "", "Optional document reference for the operation")
	contextMemoAddRuleCmd.Flags().Bool("disabled", false, "Add the rule disabled (excluded from the due check)")

	contextMemoAddMemoCmd.Flags().String("id", "", "Memo id (kebab-case slug, unique across rules and memos)")
	contextMemoAddMemoCmd.Flags().String("summary", "", "One-line description of the reminder")
	contextMemoAddMemoCmd.Flags().String("due", "", "Due day (YYYY-MM-DD)")
	contextMemoAddMemoCmd.Flags().String("action", "", "Optional project command or hint")
	contextMemoAddMemoCmd.Flags().String("source", "", "Optional document reference")

	contextMemoDueCmd.Flags().String("today", "", "Reference day (YYYY-MM-DD; default: today, UTC)")
	contextMemoDueCmd.Flags().Bool("check", false, "Exit non-zero when something is due (for scripts)")

	contextMemoDoneCmd.Flags().String("today", "", "Reference day for a rule (YYYY-MM-DD; default: today, UTC)")

	contextMemoAddCmd.AddCommand(contextMemoAddRuleCmd, contextMemoAddMemoCmd)
	contextMemoCmd.AddCommand(contextMemoAddCmd, contextMemoListCmd, contextMemoDueCmd, contextMemoDoneCmd, contextMemoRemoveCmd)
}
