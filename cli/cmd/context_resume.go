package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// `sdt context resume` is the read-only session-start view of the **recorded**
// execution state: which plan is open, what each of its phases holds, what is
// blocked and why, and what the reconciler found stale or orphaned. It renders
// the same reconcileCorpus report as `sdt context lint`, so the view and the
// findings can never disagree.

// ctxResumePremise is the caveat the view states in every format. It is the
// honest limit of the instrument: this command reads what the agent wrote, so a
// session that wrote nothing is indistinguishable from one that never started.
const ctxResumePremise = "recorded state only: a session that wrote nothing looks the same as one that never started"

// resumePlan is one open plan as the view renders it.
type resumePlan struct {
	Ref     string              `json:"ref" yaml:"ref"`
	Status  string              `json:"status" yaml:"status"`
	Tasks   []string            `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Phases  []reconcilePhase    `json:"phases,omitempty" yaml:"phases,omitempty"`
	Blocked []resumePlanBlocked `json:"blocked,omitempty" yaml:"blocked,omitempty"`
}

// resumePlanBlocked is one blocked item with the task file it lives in.
type resumePlanBlocked struct {
	Task   string `json:"task" yaml:"task"`
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Text   string `json:"text" yaml:"text"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// resumeStale is one task file past the staleness window.
type resumeStale struct {
	Task    string `json:"task" yaml:"task"`
	Status  string `json:"status" yaml:"status"`
	AgeDays int    `json:"age_days" yaml:"age_days"`
}

// resumeOrphan is one task file belonging to no plan, with the reason it is
// unexplained.
type resumeOrphan struct {
	Task   string `json:"task" yaml:"task"`
	Reason string `json:"reason" yaml:"reason"`
}

// resumeView is the whole rendered view, in the shape --format json|yaml emits.
type resumeView struct {
	Premise string           `json:"premise" yaml:"premise"`
	Stale   []resumeStale    `json:"stale" yaml:"stale"`
	Orphans []resumeOrphan   `json:"orphans" yaml:"orphans"`
	Plans   []resumePlan     `json:"plans" yaml:"plans"`
	Drift   []reconcileDrift `json:"drift,omitempty" yaml:"drift,omitempty"`
}

// buildResumeView reconciles the corpus and shapes the report for display. An
// explicit planRef selects that one plan; empty selects every active plan.
func buildResumeView(planRef string, staleDays int) resumeView {
	rep := reconcileCorpus(staleDays, contextNow())
	view := resumeView{Premise: ctxResumePremise, Stale: []resumeStale{}, Orphans: []resumeOrphan{}}

	for _, t := range rep.StaleTasks() {
		view.Stale = append(view.Stale, resumeStale{Task: t.Ref, Status: t.Status, AgeDays: t.AgeDays})
	}
	for _, t := range rep.Orphans() {
		reason := "no `plan_id` and no standalone record"
		if t.PlanID != "" {
			reason = fmt.Sprintf("`plan_id` %q does not resolve to a plan document", t.PlanID)
		}
		view.Orphans = append(view.Orphans, resumeOrphan{Task: t.Ref, Reason: reason})
	}
	for _, plan := range rep.Plans {
		if planRef != "" && !resumePlanMatches(planRef, plan.Ref) {
			continue
		}
		if planRef == "" && plan.Status != ctxWikiStatusActive {
			continue
		}
		tasks := rep.TasksOfPlan(plan.Ref)
		rp := resumePlan{Ref: plan.Ref, Status: plan.Status}
		for _, t := range tasks {
			rp.Tasks = append(rp.Tasks, t.Ref)
			rp.Phases = append(rp.Phases, t.Phases...)
			for _, b := range t.Blocked {
				rp.Blocked = append(rp.Blocked, resumePlanBlocked{Task: t.Ref, ID: b.ID, Text: b.Text, Reason: b.Reason})
			}
		}
		view.Plans = append(view.Plans, rp)
	}
	view.Drift = rep.Drift()
	return view
}

// outputResumeView renders the view in the requested format. The text renderer
// is the one a session reads at start; the structured one carries the same
// fields so a script never has to parse the prose.
func outputResumeView(cmd *cobra.Command, view resumeView) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(view, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(view)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		writeResumeText(cmd, view)
	}
}

// writeResumeText renders the session-start view: every active plan with its
// phase counts, then the blocked items with their recorded reasons, then the
// stale and orphaned files, and finally the premise that bounds what the whole
// view can claim.
func writeResumeText(cmd *cobra.Command, view resumeView) {
	for _, p := range view.Plans {
		outputString(cmd, fmt.Sprintf("plan  %s  [%s]\n", p.Ref, p.Status))
		for _, ph := range p.Phases {
			outputString(cmd, fmt.Sprintf("  phase %-6s %d/%d done", phaseLabel(ph), ph.Done, ph.Total))
			if ph.Wip > 0 {
				outputString(cmd, fmt.Sprintf("  · %d in progress", ph.Wip))
			}
			if ph.Blocked > 0 {
				outputString(cmd, fmt.Sprintf("  · %d blocked", ph.Blocked))
			}
			outputString(cmd, "\n")
		}
		for _, b := range p.Blocked {
			line := fmt.Sprintf("  blocked %s  %s", itemLabel(b.ID), truncateResumeText(b.Text))
			if b.Reason != "" {
				line += fmt.Sprintf(" — %s", b.Reason)
			}
			outputString(cmd, line+"\n")
		}
	}
	if len(view.Plans) == 0 {
		outputString(cmd, "no active plan recorded\n")
	}
	if len(view.Stale) > 0 {
		outputString(cmd, "\nstale task files\n")
		for _, s := range view.Stale {
			outputString(cmd, fmt.Sprintf("  %s  [%s, %d day(s) without an update]\n", s.Task, s.Status, s.AgeDays))
		}
	}
	if len(view.Orphans) > 0 {
		outputString(cmd, "\norphan task files\n")
		for _, o := range view.Orphans {
			outputString(cmd, fmt.Sprintf("  %s  — %s\n", o.Task, o.Reason))
		}
	}
	outputString(cmd, "\n"+ctxResumePremise+"\n")
}

// phaseLabel names a phase in the text output; an unphased legacy checklist gets
// a readable label instead of a blank column.
func phaseLabel(ph reconcilePhase) string {
	if ph.Label == "" {
		return "unphased"
	}
	return ph.Label
}

// itemLabel names a checklist item, falling back to a dash when the document
// carries no anchor.
func itemLabel(id string) string {
	if id == "" {
		return "-"
	}
	return id
}

// truncateResumeText shortens an item text to one readable line for the text
// renderer; the structured output keeps the full text.
func truncateResumeText(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) <= 72 {
		return flat
	}
	return flat[:69] + "..."
}

// resumePlanMatches reports whether an explicit --plan reference selects a plan.
// It accepts every form the task family accepts — a full corpus reference
// (`context/plan/<file>.md`), a `plan/<file>.md` path, a bare filename or the
// plan slug — because a caller typing a plan reference should not have to know
// which of those a command happens to want. An unmatched reference selects
// nothing rather than falling back to every active plan.
func resumePlanMatches(ref, planRef string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if normalizeContextRef(ref) == planRef || filepath.Base(ref) == filepath.Base(planRef) {
		return true
	}
	return taskSlugFromPlan(filepath.Base(ref)) == taskSlugFromPlan(filepath.Base(planRef))
}

var contextResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Show the recorded execution state to resume from (read-only)",
	Long: `Show what the context/ documents say about the work in flight: the open
plan(s), each task file's phases with item counts, the blocked items with the
reason recorded on them, and what the reconciler found stale or orphaned.

` + ctxResumePremise + `. Nothing here is a live process view: it reports the
paperwork, so use it to decide what to open next, never to claim what is running.

Examples:
  sdt context resume
  sdt context resume --plan 20261005-example.md
  sdt context resume --stale-days 30
  sdt context resume --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		planRef := getStringFlag(cmd, "plan", false)
		view := buildResumeView(planRef, getIntFlag(cmd, "stale-days", false))
		outputResumeView(cmd, view)
	},
}
