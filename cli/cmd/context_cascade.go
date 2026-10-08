package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

// The cascade derives completion upward: a task file from its own checklist, a
// plan from its sourcing task files (plus its own checklists), an analysis from
// its sourcing plans. `completed` is the only terminal status that counts;
// `archived` never feeds the cascade and a document with no children is never
// auto-flipped. One reconciler backs both cascade-on-write and `context sync`.

// cascadeNode is one derived document (a task file, plan or analysis) with the
// data the reconciler needs.
type cascadeNode struct {
	path      string
	kind      string
	ref       string // normalizeContextRef(path)
	status    string
	uid       string
	parentRef string // task→plan or plan→analysis, resolved by ctxrel; "" when none
	content   string
	items     []checklistItem
}

// cascadeChange is one applied (or would-be) status flip.
type cascadeChange struct {
	Path string `json:"path" yaml:"path"`
	Kind string `json:"kind" yaml:"kind"`
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

// cascadeStore indexes the derived documents by normalized ref and carries the
// resolved lifecycle edges.
type cascadeStore struct {
	tasks    map[string]*cascadeNode
	plans    map[string]*cascadeNode
	analyses map[string]*cascadeNode
	edges    *ctxrel.Edges
}

func newCascadeStore() *cascadeStore {
	return &cascadeStore{
		tasks:    map[string]*cascadeNode{},
		plans:    map[string]*cascadeNode{},
		analyses: map[string]*cascadeNode{},
	}
}

func loadCascadeNode(path string) (*cascadeNode, bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return nil, false
	}
	content := string(data)
	kind := strings.TrimSpace(parseFrontmatterField(content, "kind"))
	switch kind {
	case ctxTypeTasks, ctxTypePlan, ctxTypeAnalysis:
	default:
		return nil, false
	}
	n := &cascadeNode{
		path:    path,
		kind:    kind,
		ref:     normalizeContextRef(path),
		status:  strings.ToLower(strings.TrimSpace(parseFrontmatterField(content, ctxMapStatus))),
		uid:     parseFrontmatterField(content, ctxFrontmatterUID),
		content: content,
		// Only the work part: the derived status behind the cascade checks must
		// not read a `## Deviations`/`## Review` record as open work.
		items: parseChecklistItems(taskWorkContent(content)),
	}
	return n, true
}

func walkCascadeDir(dir string, add func(*cascadeNode)) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != sdtMarkdownExt {
			return nil
		}
		if n, ok := loadCascadeNode(path); ok {
			add(n)
		}
		return nil
	})
}

func loadCascadeStore() (*cascadeStore, error) {
	s := newCascadeStore()
	// The lifecycle edges are the typed relations (`analysis_id`, `plan_id`),
	// resolved once by ctxrel; a plan derives from exactly one analysis and a
	// task file from exactly one plan.
	edges, err := ctxrel.Load(sdtWorkDir)
	if err != nil {
		return nil, err
	}
	s.edges = edges
	// parent-to-child, so a node always has its parent available for the
	// in-memory reconciliation below.
	byDir := []struct {
		dir  string
		into map[string]*cascadeNode
	}{
		{sdtAnalysisDir, s.analyses},
		{sdtPlanDir, s.plans},
		{sdtTasksDir, s.tasks},
	}
	for _, d := range byDir {
		if err := walkCascadeDir(d.dir, func(n *cascadeNode) {
			n.parentRef = edges.ParentOf(n.ref)
			d.into[n.ref] = n
		}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// deriveTaskStatus computes a task file status from its own checklist, or ""
// when the file has no checklist (never auto-flipped).
func deriveTaskStatus(n *cascadeNode) string {
	if len(n.items) == 0 {
		return ""
	}
	unfinished, started := 0, false
	for _, it := range n.items {
		if it.Status != taskStatusDone {
			unfinished++
		}
		if it.Status != taskStatusTodo {
			started = true
		}
	}
	switch {
	case unfinished == 0:
		return taskFileStatusCompleted
	case started:
		return taskFileStatusInProgress
	default:
		return taskFileStatusPending
	}
}

// childTasks returns the task nodes whose typed parent is a plan, sorted by path.
func (s *cascadeStore) childTasks(planRef string) []*cascadeNode {
	return s.childrenOf(planRef, s.tasks)
}

// childPlans returns the plan nodes whose typed parent is an analysis, sorted by
// path.
func (s *cascadeStore) childPlans(analysisRef string) []*cascadeNode {
	return s.childrenOf(analysisRef, s.plans)
}

// childrenOf reads the resolved edges and returns the matching nodes, sorted by
// path. A document whose parent could not be resolved belongs to no group.
func (s *cascadeStore) childrenOf(parentRef string, byRef map[string]*cascadeNode) []*cascadeNode {
	out := make([]*cascadeNode, 0, len(s.edges.ChildrenOf(parentRef)))
	for _, ref := range s.edges.ChildrenOf(parentRef) {
		if n, ok := byRef[ref]; ok {
			out = append(out, n)
		}
	}
	return out
}

// effectiveStatus is a task's derived status when derivable, else its declared
// status (lowercased).
func (n *cascadeNode) effectiveStatus() string {
	if n.kind == ctxTypeTasks {
		if d := deriveTaskStatus(n); d != "" {
			return d
		}
	}
	return n.status
}

// derivePlanStatus returns a plan's derived status, or "" when it has no child
// task file (never auto-flipped). It is completed when every child task is done
// and the plan's own checklists are all done, active otherwise.
func (s *cascadeStore) derivePlanStatus(n *cascadeNode) string {
	children := s.childTasks(n.ref)
	if len(children) == 0 {
		return ""
	}
	for _, c := range children {
		if c.effectiveStatus() != taskFileStatusCompleted {
			return ctxWikiStatusActive
		}
	}
	// Reference-only model (F13): the plan's `## Phases` / `## Completion
	// criteria` checkboxes are intent, not a second executed checklist, so they
	// never gate completion — the task files own it.
	return taskFileStatusCompleted
}

// deriveAnalysisStatus returns an analysis' derived status, or "" when it has
// no child plan (never auto-flipped): completed when every child plan is done.
func (s *cascadeStore) deriveAnalysisStatus(n *cascadeNode) string {
	children := s.childPlans(n.ref)
	if len(children) == 0 {
		return ""
	}
	for _, c := range children {
		if c.effectiveStatus() != taskFileStatusCompleted {
			return ctxWikiStatusActive
		}
	}
	return taskFileStatusCompleted
}

// statusFlappable reports whether the reconciler may rewrite a node from its
// current status (archived/draft/abandoned states are user-owned).
func statusFlappable(n *cascadeNode) bool {
	switch n.kind {
	case ctxTypeTasks:
		return n.status != taskFileStatusArchived
	case ctxTypePlan:
		return n.status == ctxWikiStatusActive || n.status == taskFileStatusCompleted
	case ctxTypeAnalysis:
		return n.status == ctxWikiStatusActive || n.status == taskFileStatusCompleted
	default:
		return false
	}
}

// statusRank orders the derived statuses from least to most advanced; -1 marks
// a status outside the cascade's ladder.
func statusRank(kind, status string) int {
	switch kind {
	case ctxTypeTasks:
		switch status {
		case taskFileStatusPending:
			return 0
		case taskFileStatusInProgress:
			return 1
		case taskFileStatusCompleted:
			return 2
		}
	case ctxTypePlan, ctxTypeAnalysis:
		switch status {
		case ctxWikiStatusActive:
			return 0
		case taskFileStatusCompleted:
			return 1
		}
	}
	return -1
}

// advances reports whether the reconciler may move a document from `from` to
// `to`. The cascade only advances (pending→in-progress→completed,
// active→completed); regressions and declared-vs-derived conflicts are reported
// by `context lint`, never silently rewritten.
func advances(kind, from, to string) bool {
	rf, rt := statusRank(kind, from), statusRank(kind, to)
	return rf >= 0 && rt > rf
}

// applyDerivedStatus rewrites the node's frontmatter status (and `updated`) and
// returns the refreshed node.
func applyDerivedStatus(n *cascadeNode, status string) error {
	patches := []frontmatterPatch{{key: ctxMapStatus, value: status}, {key: statusUpdated, value: contextNow().UTC().Format(time.RFC3339)}}
	content, changed := setFrontmatterFields(n.content, patches)
	if !changed {
		return fmt.Errorf("no status field written to %s (missing frontmatter)", n.path)
	}
	if err := writeWorkFile(n.path, content); err != nil {
		return err
	}
	n.content = content
	n.status = status
	return nil
}

// cascadeUp reconciles from startRef upward (task→plan→analysis), applying the
// flips when apply is true and returning every change (applied or, with
// dryRun, merely reported).
func cascadeUp(startRef string, apply bool) ([]cascadeChange, error) {
	store, err := loadCascadeStore()
	if err != nil {
		return nil, err
	}
	node := store.byRef(startRef)
	if node == nil {
		return nil, nil
	}
	var changes []cascadeChange
	for node != nil {
		if !statusFlappable(node) {
			break
		}
		var derived string
		switch node.kind {
		case ctxTypeTasks:
			derived = deriveTaskStatus(node)
		case ctxTypePlan:
			derived = store.derivePlanStatus(node)
		case ctxTypeAnalysis:
			derived = store.deriveAnalysisStatus(node)
		}
		if derived != "" && derived != node.status && advances(node.kind, node.status, derived) {
			changes = append(changes, cascadeChange{Path: node.path, Kind: node.kind, From: node.status, To: derived})
			if apply {
				if err := applyDerivedStatus(node, derived); err != nil {
					return changes, err
				}
			}
		}
		if node.parentRef == "" {
			break
		}
		// The parent's derivation reads the child's refreshed status from the
		// in-memory node, so the loop must continue from the store entry.
		node = store.byRef(node.parentRef)
	}
	return changes, nil
}

func (s *cascadeStore) byRef(ref string) *cascadeNode {
	if n, ok := s.tasks[ref]; ok {
		return n
	}
	if n, ok := s.plans[ref]; ok {
		return n
	}
	return s.analyses[ref]
}

// lintCascadeDrift reports declared-vs-derived drift across the chain as
// advisory WARNINGs: a task file whose checklist disagrees with its status, an
// analysis whose plans disagree, and the "derivably completed" adoption case
// for plans and analyses. The plan-declares-completed-but-unfinished direction
// stays with lintPlanTaskAgreement (which mirrors the viewer's done vocabulary).
func lintCascadeDrift() []ctxLintIssue {
	store, err := loadCascadeStore()
	if err != nil {
		return nil
	}
	var issues []ctxLintIssue
	for _, n := range sortedNodes(store.tasks) {
		if !statusFlappable(n) {
			continue
		}
		d := deriveTaskStatus(n)
		if d == "" || d == n.status {
			continue
		}
		switch {
		case n.status == taskFileStatusCompleted && d != taskFileStatusCompleted:
			issues = append(issues, ctxLintIssue{Path: n.path, Priority: ctxLintWarning, Message: "task declares completed but its checklist has unfinished items (tick or reopen them, then run `sdt context sync`)"})
		case n.status != taskFileStatusCompleted && d == taskFileStatusCompleted:
			issues = append(issues, ctxLintIssue{Path: n.path, Priority: ctxLintWarning, Message: "task checklist is complete but the file status is `" + n.status + "` (run `sdt context sync`)"})
		}
	}
	for _, n := range sortedNodes(store.plans) {
		if n.status != ctxWikiStatusActive {
			continue
		}
		if store.derivePlanStatus(n) == taskFileStatusCompleted {
			issues = append(issues, ctxLintIssue{Path: n.path, Priority: ctxLintWarning, Message: "plan is derivably completed (all task files done and its own checklists) — run `sdt context sync`"})
		}
	}
	for _, n := range sortedNodes(store.analyses) {
		if !statusFlappable(n) {
			continue
		}
		d := store.deriveAnalysisStatus(n)
		if d == "" || d == n.status {
			continue
		}
		if n.status == taskFileStatusCompleted {
			unfinished := analysisUnfinishedPlans(store, n)
			issues = append(issues, ctxLintIssue{Path: n.path, Priority: ctxLintWarning, Message: fmt.Sprintf("analysis declares completed but %d plan(s) are not done: %s", len(unfinished), strings.Join(unfinished, ", "))})
		} else {
			issues = append(issues, ctxLintIssue{Path: n.path, Priority: ctxLintWarning, Message: "analysis is derivably completed (all plans done) — run `sdt context sync`"})
		}
	}
	return issues
}

// derivedCompletionBlock returns the derived status for a document when it
// disagrees with a declared `completed` (a non-terminal derived value), plus a
// human reason. It returns "" when there is no derivation or the derived status
// is already completed (so the transition is allowed).
func derivedCompletionBlock(path string) (string, string) {
	store, err := loadCascadeStore()
	if err != nil {
		return "", ""
	}
	n := store.byRef(normalizeContextRef(path))
	if n == nil {
		return "", ""
	}
	var derived string
	switch n.kind {
	case ctxTypeTasks:
		derived = deriveTaskStatus(n)
	case ctxTypePlan:
		derived = store.derivePlanStatus(n)
	case ctxTypeAnalysis:
		derived = store.deriveAnalysisStatus(n)
	default:
		return "", ""
	}
	if derived == "" || derived == taskFileStatusCompleted {
		return "", ""
	}
	return derived, fmt.Sprintf("derived state is %q", derived)
}

// analysisUnfinishedPlans lists the base names of an analysis' plans that are
// not done.
func analysisUnfinishedPlans(store *cascadeStore, n *cascadeNode) []string {
	var out []string
	for _, c := range store.childPlans(n.ref) {
		if c.effectiveStatus() != taskFileStatusCompleted {
			out = append(out, filepath.Base(c.path))
		}
	}
	return out
}

// outputCascadeChanges prints the flips a command triggered. It stays silent
// under json/yaml so it never corrupts the caller's structured output.
func outputCascadeChanges(cmd *cobra.Command, changes []cascadeChange) {
	if len(changes) == 0 {
		return
	}
	switch getFormat(cmd) {
	case fmtJSON, fmtYAML:
		return
	default:
		for _, c := range changes {
			outputString(cmd, fmt.Sprintf("%s: %s → %s\n", c.Path, c.From, c.To))
		}
	}
}

// cascadeAfterWrite propagates a just-written document's state to its parents
// (unless --no-cascade) and reports the flips.
func cascadeAfterWrite(cmd *cobra.Command, path string) {
	if !cascadeEnabled(cmd) {
		return
	}
	changes, err := cascadeUp(path, true)
	exitWithError(cmd, err)
	outputCascadeChanges(cmd, changes)
}

func cascadeEnabled(cmd *cobra.Command) bool {
	if f := cmd.Flags().Lookup("no-cascade"); f != nil {
		return !getBoolFlag(cmd, "no-cascade", false)
	}
	return true
}

func addCascadeFlag(c *cobra.Command) {
	c.Flags().Bool("no-cascade", false, "Do not propagate completion to parent documents")
}
