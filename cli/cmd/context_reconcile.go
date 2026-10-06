package cmd

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

// The reconciler: the corpus-wide plan<->task cross-checks that answer "is this
// plan's paperwork telling the truth?". Every check here reads **recorded** state
// only (decision 0023: SDT is an instrument, never a controller of the agent), so
// each one is advisory and none of them refuses an action.
//
// `reconcileCorpus` is the single computation behind all of it. `sdt context
// lint` renders it as findings and `sdt context resume` renders it as a view, so
// the two surfaces can never disagree about what the corpus says.

// ctxStaleInProgressDays is the default window after which a task file left
// `in-progress` is reported as stale. It is a named constant rather than a
// literal in the comparison so `--stale-days` has one source to override.
const ctxStaleInProgressDays = 14

// ctxDay is the staleness unit the window is expressed in.
const ctxDay = 24 * time.Hour

// ctxStandaloneMarker is the body sentence that records a task file with no
// parent plan on purpose (`sdt context task --plan <custom-slug>` scaffolds one
// for a process record that must not ride a plan's lifecycle). The marker is a
// documented convention in prose rather than a frontmatter key, so a standalone
// file states the decision where a reader already looks and the check reads the
// same sentence. Its absence is the finding: an unmarked orphan is silent.
const ctxStandaloneMarker = "Standalone task file:"

// ctxBlockedReasonRegexp extracts the reason `sdt context task block --reason`
// appends to a blocked item: `- [!] text (blocked: waiting on X) <!-- c3 -->`.
var ctxBlockedReasonRegexp = regexp.MustCompile(`\s*\(blocked:\s*(.+)\)\s*$`)

// reconcilePlan is one plan document as the reconciler sees it: its status and
// the task files that resolve to it.
type reconcilePlan struct {
	Ref      string   `json:"ref" yaml:"ref"`
	Path     string   `json:"path" yaml:"path"`
	Status   string   `json:"status" yaml:"status"`
	TaskRefs []string `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

// reconcilePhase counts the checklist items of one `## Phase <label>` section.
type reconcilePhase struct {
	Label   string `json:"label" yaml:"label"`
	Total   int    `json:"total" yaml:"total"`
	Done    int    `json:"done" yaml:"done"`
	Wip     int    `json:"wip" yaml:"wip"`
	Blocked int    `json:"blocked" yaml:"blocked"`
}

// reconcileBlocked is one blocked checklist item and the reason recorded with it.
type reconcileBlocked struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Text   string `json:"text" yaml:"text"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// reconcileTask is one task file as the reconciler sees it: the frontmatter
// state, the declared and resolved parent plan, the per-phase item counts and
// the blocked items.
type reconcileTask struct {
	Path       string `json:"path" yaml:"path"`
	Ref        string `json:"ref" yaml:"ref"`
	Status     string `json:"status" yaml:"status"`
	Updated    string `json:"updated,omitempty" yaml:"updated,omitempty"`
	AgeDays    int    `json:"age_days" yaml:"age_days"`
	PlanID     string `json:"plan_id,omitempty" yaml:"plan_id,omitempty"`
	PlanRef    string `json:"plan,omitempty" yaml:"plan,omitempty"`
	Standalone bool   `json:"standalone,omitempty" yaml:"standalone,omitempty"`
	Stale      bool   `json:"stale,omitempty" yaml:"stale,omitempty"`
	HasReview  bool   `json:"has_review,omitempty" yaml:"has_review,omitempty"`
	HasGate    bool   `json:"has_gate,omitempty" yaml:"has_gate,omitempty"`
	// UpdatedAt is the parsed `updated` timestamp, kept unexported to the
	// serialized forms: the record reports AgeDays, the checks compare the time.
	UpdatedAt time.Time          `json:"-" yaml:"-"`
	Phases    []reconcilePhase   `json:"phases,omitempty" yaml:"phases,omitempty"`
	Blocked   []reconcileBlocked `json:"blocked,omitempty" yaml:"blocked,omitempty"`
}

// reconcileReport is the whole plan<->task reconciliation of the current project.
type reconcileReport struct {
	Now       time.Time       `json:"now" yaml:"now"`
	StaleDays int             `json:"stale_days" yaml:"stale_days"`
	Plans     []reconcilePlan `json:"plans" yaml:"plans"`
	Tasks     []reconcileTask `json:"tasks" yaml:"tasks"`
}

// reconcileCorpus reads the plan and task documents once and reconciles them
// against each other: the typed parent edges from ctxrel, the per-phase item
// counts, the blocked items with their recorded reasons, the stale files, the
// orphans and the status drift between a plan and its task files. A non-positive
// staleDays falls back to ctxStaleInProgressDays.
func reconcileCorpus(staleDays int, now time.Time) *reconcileReport {
	if staleDays <= 0 {
		staleDays = ctxStaleInProgressDays
	}
	// A corpus that cannot be read reconciles as empty: the report is read-only
	// and advisory, so it degrades rather than failing the command.
	planFiles, err := dirFiles(sdtPlanDir)
	if err != nil {
		planFiles = nil
	}
	taskFiles, err := dirFiles(sdtTasksDir)
	if err != nil {
		taskFiles = nil
	}
	edges, err := ctxrel.Load(sdtWorkDir)
	if err != nil {
		edges = &ctxrel.Edges{}
	}

	rep := &reconcileReport{Now: now.UTC(), StaleDays: staleDays}
	tasksByPlan := map[string][]int{}
	for _, path := range taskFiles {
		task, ok := reconcileTaskDoc(path, edges, now, staleDays)
		if !ok {
			continue
		}
		if task.PlanRef != "" {
			tasksByPlan[task.PlanRef] = append(tasksByPlan[task.PlanRef], len(rep.Tasks))
		}
		rep.Tasks = append(rep.Tasks, task)
	}
	for _, path := range planFiles {
		content, ok := readReconcileDoc(path)
		if !ok || parseFrontmatterField(content, "kind") != ctxTypePlan {
			continue
		}
		ref := normalizeContextRef(path)
		plan := reconcilePlan{
			Ref:    ref,
			Path:   path,
			Status: strings.ToLower(strings.TrimSpace(parseFrontmatterField(content, "status"))),
		}
		for _, idx := range tasksByPlan[ref] {
			plan.TaskRefs = append(plan.TaskRefs, rep.Tasks[idx].Ref)
		}
		rep.Plans = append(rep.Plans, plan)
	}
	return rep
}

// reconcileTaskDoc reconciles one task file against the typed relations, the
// clock and the staleness window. It reports false for a file that is not a task
// document.
func reconcileTaskDoc(path string, edges *ctxrel.Edges, now time.Time, staleDays int) (reconcileTask, bool) {
	content, ok := readReconcileDoc(path)
	if !ok || parseFrontmatterField(content, "kind") != ctxTypeTasks {
		return reconcileTask{}, false
	}
	task := reconcileTask{
		Path:       path,
		Ref:        normalizeContextRef(path),
		Status:     strings.ToLower(strings.TrimSpace(parseFrontmatterField(content, "status"))),
		PlanID:     strings.TrimSpace(parseFrontmatterField(content, ctxKeyPlanID)),
		PlanRef:    edges.ParentOf(normalizeContextRef(path)),
		Standalone: declaresStandaloneTask(content),
		HasReview:  hasReviewBlock(content),
		HasGate:    hasGateRecord(content),
	}
	if v := parseFrontmatterField(content, statusUpdated); v != "" {
		if t, perr := time.Parse(time.RFC3339, v); perr == nil {
			task.Updated = t.UTC().Format(time.RFC3339)
			task.UpdatedAt = t.UTC()
			age := now.Sub(t)
			task.AgeDays = int(age / ctxDay)
			task.Stale = isOpenTaskStatus(task.Status) && age > time.Duration(staleDays)*ctxDay
		}
	}
	task.Phases, task.Blocked = reconcilePhases(content)
	return task, true
}

// isOpenTaskStatus reports whether a task file status means "work is still
// happening here": an unfinished file is the only one that can go stale.
func isOpenTaskStatus(status string) bool {
	return status == taskFileStatusInProgress || status == taskFileStatusLegacy
}

// reconcilePhases splits a task file into its `## Phase <label>` sections and
// counts the items in each, plus the blocked items with their recorded reasons.
// Items before the first phase heading land in the preamble section, so an
// unphased legacy checklist is still fully accounted for.
func reconcilePhases(content string) ([]reconcilePhase, []reconcileBlocked) {
	lines := strings.Split(content, "\n")
	// Start line of every section in document order; the preamble section starts
	// at line 0 under the empty label, so an item before the first `## Phase`
	// heading is accounted for rather than dropped.
	starts := []int{0}
	labels := []string{""}
	for i, line := range lines {
		if m := ctxTaskPhaseSectionRegexp.FindStringSubmatch(line); m != nil {
			starts = append(starts, i)
			labels = append(labels, m[1])
		}
	}
	starts = append(starts, len(lines))
	counts := map[string]*reconcilePhase{}
	for _, label := range labels {
		counts[label] = &reconcilePhase{Label: label}
	}
	var blocked []reconcileBlocked
	// Entries (not parseChecklistItems) because only they carry the absolute
	// document line the section boundaries are compared against.
	for _, e := range parseChecklistEntries(lines) {
		section := ""
		for i := 0; i+1 < len(starts); i++ {
			if e.First >= starts[i] && e.First < starts[i+1] {
				section = labels[i]
				break
			}
		}
		body, _ := splitChecklistAnchor(e.Body)
		p := counts[section]
		p.Total++
		switch checklistStatus(e.Marker) {
		case taskStatusDone:
			p.Done++
		case taskStatusWip:
			p.Wip++
		case taskStatusBlocked:
			p.Blocked++
			blocked = append(blocked, reconcileBlocked{ID: e.ID, Text: reconcileItemText(body), Reason: blockedReason(body)})
		}
	}
	var phases []reconcilePhase
	for _, label := range sortedPhaseLabels(counts) {
		if counts[label].Total == 0 {
			continue
		}
		phases = append(phases, *counts[label])
	}
	return phases, blocked
}

// sortedPhaseLabels returns the section labels in document order, with the
// preamble (empty label) first.
func sortedPhaseLabels(counts map[string]*reconcilePhase) []string {
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool {
		li, lj := labels[i], labels[j]
		if (li == "") != (lj == "") {
			return li == ""
		}
		if li == "" {
			return false
		}
		return phaseLabelLess(li, lj)
	})
	return labels
}

// phaseLabelLess orders two phase labels numerically when both are numbers, so
// phase 10 sorts after phase 9, and lexically otherwise.
func phaseLabelLess(a, b string) bool {
	na, erra := strconv.Atoi(a)
	nb, errb := strconv.Atoi(b)
	if erra == nil && errb == nil {
		return na < nb
	}
	return a < b
}

// blockedReason returns the reason recorded on a blocked item, or "".
func blockedReason(text string) string {
	if m := ctxBlockedReasonRegexp.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// reconcileItemText is the item text without the trailing `(blocked: ...)`
// suffix, which the reason already carries.
func reconcileItemText(text string) string {
	return strings.TrimSpace(ctxBlockedReasonRegexp.ReplaceAllString(text, ""))
}

// TasksOfPlan returns the task files resolved to a plan reference.
func (r *reconcileReport) TasksOfPlan(planRef string) []reconcileTask {
	var out []reconcileTask
	for _, t := range r.Tasks {
		if t.PlanRef == planRef {
			out = append(out, t)
		}
	}
	return out
}

// Orphans returns the task files that belong to no plan and did not record the
// standalone decision — the exact set lintTaskOrphans reports.
func (r *reconcileReport) Orphans() []reconcileTask {
	var out []reconcileTask
	for _, t := range r.Tasks {
		if t.PlanRef == "" && !t.Standalone {
			out = append(out, t)
		}
	}
	return out
}

// StaleTasks returns the open task files past the staleness window.
func (r *reconcileReport) StaleTasks() []reconcileTask {
	var out []reconcileTask
	for _, t := range r.Tasks {
		if t.Stale {
			out = append(out, t)
		}
	}
	return out
}

// Drift returns the status disagreements between a plan and its task files: a
// plan still `active` whose task files are all completed, and the completed task
// files under such a plan. The reverse direction is reported only once the plan
// has no open work left: while a sibling task file is still `pending` or
// `in-progress`, a completed phase file is the normal state of a plan being
// executed phase by phase, not drift (`Ruling` 1 recorded in the Wave 4 task
// file — a finding per finished phase of a live plan would bury the one that
// matters).
func (r *reconcileReport) Drift() []reconcileDrift {
	var out []reconcileDrift
	for _, plan := range r.Plans {
		if plan.Status != ctxWikiStatusActive {
			continue
		}
		tasks := r.TasksOfPlan(plan.Ref)
		if len(tasks) == 0 {
			continue
		}
		allDone := true
		for _, t := range tasks {
			if !ctxDoneStatuses[t.Status] {
				allDone = false
				break
			}
		}
		if !allDone {
			continue
		}
		out = append(out, reconcileDrift{Path: plan.Path, Kind: "plan-open",
			Message: fmt.Sprintf("plan status is %q but all %d task file(s) are completed", plan.Status, len(tasks))})
		for _, t := range tasks {
			if ctxDoneStatuses[t.Status] {
				out = append(out, reconcileDrift{Path: t.Path, Kind: "task-closed",
					Message: "task file is completed but its parent plan is still active"})
			}
		}
	}
	return out
}

// reconcileDrift is one status disagreement, addressed at the document whose
// recorded status is the one to change.
type reconcileDrift struct {
	Path    string `json:"path" yaml:"path"`
	Kind    string `json:"kind" yaml:"kind"`
	Message string `json:"message" yaml:"message"`
}

// lintTaskOrphans reports a task file that belongs to no plan: its `plan_id` is
// absent, or names a uid no plan document carries. Such a file is invisible to
// every plan-scoped check (they all key on the resolved parent), so it silently
// escapes the reconciler. WARNING: the paperwork is incomplete, not wrong —
// `sdt context relations backfill` is the remediation.
func lintTaskOrphans(rep *reconcileReport) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, t := range rep.Orphans() {
		msg := "orphan task file: no `plan_id` (belongs to no plan, so no plan-scoped check can see it)"
		if t.PlanID != "" {
			msg = fmt.Sprintf("orphan task file: `plan_id` %q does not resolve to a plan document", t.PlanID)
		}
		issues = append(issues, ctxLintIssue{Path: t.Path, Priority: ctxLintWarning, Message: msg})
	}
	return issues
}

// lintPlanTaskStatusDrift reports a plan/task pair whose statuses disagree with
// the work actually done. SUGGESTION — a signal that the paper was not updated
// at the end of a phase, never an error in the code.
func lintPlanTaskStatusDrift(rep *reconcileReport) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, d := range rep.Drift() {
		issues = append(issues, ctxLintIssue{Path: d.Path, Priority: ctxLintSuggestion, Message: d.Message})
	}
	return issues
}

// lintStaleInProgress reports a task file left `in-progress` (or the legacy
// `active`) with no update for longer than the staleness window. The window is
// compared against the frontmatter `updated` (RFC3339 UTC, already lint
// validated) and `contextNow`, never the file mtime: the recorded timestamp is
// the state the CLI owns. SUGGESTION — an agent may legitimately hold a task
// open for weeks; the point is that the next session can see it.
func lintStaleInProgress(rep *reconcileReport) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, t := range rep.StaleTasks() {
		issues = append(issues, ctxLintIssue{Path: t.Path, Priority: ctxLintSuggestion,
			Message: fmt.Sprintf("stale task file: status %s with no update for %d day(s) (>%d); receive it or reset its `[~]` items",
				t.Status, t.AgeDays, rep.StaleDays)})
	}
	return issues
}

// declaresStandaloneTask reports whether the body carries the standalone marker.
func declaresStandaloneTask(content string) bool {
	return strings.Contains(content, ctxStandaloneMarker)
}

// hasGateRecord reports whether a task file carries a gate record, wherever the
// `### Gate` subsection sits.
func hasGateRecord(content string) bool {
	lines := strings.Split(content, "\n")
	return indexOfTrimmedLine(lines, ctxGateRecordHeader, 0, len(lines)) >= 0
}

// PlanStatus returns the recorded status of a plan reference and whether the
// plan exists at all.
func (r *reconcileReport) PlanStatus(ref string) (string, bool) {
	for _, p := range r.Plans {
		if p.Ref == ref {
			return p.Status, true
		}
	}
	return "", false
}

// PlanIsActive reports whether a reference is an active plan document.
func (r *reconcileReport) PlanIsActive(ref string) bool {
	status, ok := r.PlanStatus(ref)
	return ok && status == ctxWikiStatusActive
}

// lintGateEvidence reports a completed task file whose `## Review` block carries
// no `### Gate` record. The delivery gate is what turns "I ran the checks" into
// evidence the next session can read; a closed phase without one leaves nothing
// to verify against. SUGGESTION — the gate is a strict, optional step.
//
// Historical phases predate the feature, so the finding is skipped when the
// parent plan is no longer active *and* the file was last updated before
// ctxGateRecordSince: the one-off backlog is not a corpus-wide flood. A file
// under an active plan is always in scope, because that is the one a session
// still reads.
func lintGateEvidence(rep *reconcileReport) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, t := range rep.Tasks {
		if !ctxDoneStatuses[t.Status] || !t.HasReview || t.HasGate {
			continue
		}
		if !rep.PlanIsActive(t.PlanRef) && t.UpdatedAt.Before(ctxGateRecordSince) {
			continue
		}
		issues = append(issues, ctxLintIssue{Path: t.Path, Priority: ctxLintSuggestion,
			Message: "completed task file has a `## Review` block but no `### Gate` record; run the delivery gate and record it with `sdt agent gate --record --plan <plan>`"})
	}
	return issues
}

// readReconcileDoc reads one document for a corpus-wide check, reporting whether
// it is readable.
func readReconcileDoc(path string) (string, bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return "", false
	}
	return string(data), true
}
