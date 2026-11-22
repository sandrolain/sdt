package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Lint baseline.
//
// A corpus of a few hundred documents always has findings: a stale citation, a
// missing `objective`, a suggestion nobody disagrees with. Reporting all of them
// on every run is how "0 findings" stopped meaning anything and how a real new
// finding got lost in the noise. The baseline records the set a project has
// already accepted, so a default run answers one question — *what is new?* — and
// `--all` still shows everything.
//
// The file is versioned and lives in the corpus (context/lint-baseline.json), so
// it is reviewed like any other document: a finding enters the baseline only when
// `--baseline-write` records it deliberately.

const ctxLintBaselineFile = "context/lint-baseline.json"

// ctxLintBaselineVersion is bumped only when the shape changes; a file whose
// version is unknown is ignored rather than trusted.
const ctxLintBaselineVersion = 2

type lintBaseline struct {
	Version   int                   `json:"version"`
	Generated string                `json:"generated"`
	Findings  []lintBaselineFinding `json:"findings"`
}

type lintBaselineFinding struct {
	ID                   string              `json:"id"`
	Path                 string              `json:"path"`
	Priority             string              `json:"priority"`
	Message              string              `json:"message"`
	Status               string              `json:"status"`
	FirstSeen            string              `json:"first_seen,omitempty"`
	LastSeen             string              `json:"last_seen,omitempty"`
	ChangedAt            string              `json:"changed_at,omitempty"`
	ResolvedAt           string              `json:"resolved_at,omitempty"`
	Present              bool                `json:"present"`
	Rationale            string              `json:"rationale,omitempty"`
	Scope                string              `json:"scope,omitempty"`
	NeedsReassessment    bool                `json:"needs_reassessment,omitempty"`
	LegacyHistoryUnknown bool                `json:"legacy_history_unknown,omitempty"`
	Changes              []lintBaselineEvent `json:"changes,omitempty"`
}

type lintBaselineEvent struct {
	At     string   `json:"at"`
	Change string   `json:"change"`
	Status string   `json:"status,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

// lintFindingKey is the identity of a finding: the same document, severity and
// message is the same defect. A message that changes is a new finding on
// purpose — rewording the check should surface the reworded result.
func lintFindingKey(path, priority, message string) string {
	return path + "\x00" + priority + "\x00" + message
}

func lintFindingID(path, priority, message string) string {
	sum := sha256.Sum256([]byte(lintFindingKey(path, priority, message)))
	return hex.EncodeToString(sum[:8])
}

// loadLintBaseline reads the baseline, returning (nil, nil) when the file is
// absent — the documented "today's behaviour" case — and an error only when the
// file exists but cannot be read or parsed.
func loadLintBaseline() (*lintBaseline, error) {
	data, err := os.ReadFile(ctxLintBaselineFile) //#nosec G304 -- fixed corpus path
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var base lintBaseline
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("%s: %w", ctxLintBaselineFile, err)
	}
	if base.Version != 1 && base.Version != ctxLintBaselineVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (this build writes %d)", ctxLintBaselineFile, base.Version, ctxLintBaselineVersion)
	}
	if base.Version == 1 {
		for i := range base.Findings {
			f := &base.Findings[i]
			f.ID = lintFindingID(f.Path, f.Priority, f.Message)
			f.Status = lintBaselineUnreviewed
			f.Present = true
			f.LastSeen = base.Generated
			f.LegacyHistoryUnknown = true
		}
	}
	return &base, nil
}

func reviewLintBaseline(id, status, rationale, scope string) error {
	if status != lintBaselineUnreviewed && status != lintBaselineToFix && status != lintBaselineJustifiedException {
		return fmt.Errorf("invalid baseline status %q (use %q, %q or %q)", status, lintBaselineUnreviewed, lintBaselineToFix, lintBaselineJustifiedException)
	}
	rationale = strings.TrimSpace(rationale)
	scope = strings.TrimSpace(scope)
	if status == lintBaselineJustifiedException && (rationale == "" || scope == "") {
		return fmt.Errorf("a justified exception requires both --baseline-rationale and --baseline-scope")
	}
	base, err := loadLintBaseline()
	if err != nil {
		return err
	}
	if base == nil {
		return fmt.Errorf("%s: no baseline exists; run context lint --baseline-write first", ctxLintBaselineFile)
	}
	if base.Version != ctxLintBaselineVersion {
		return fmt.Errorf("%s: review is available after migration; run context lint --baseline-write first", ctxLintBaselineFile)
	}
	found := false
	now := contextNow().UTC().Format(time.RFC3339)
	for i := range base.Findings {
		f := &base.Findings[i]
		if f.ID == "" {
			f.ID = lintFindingID(f.Path, f.Priority, f.Message)
		}
		if f.ID != id {
			continue
		}
		if !f.Present {
			return fmt.Errorf("baseline finding %s is no longer present and cannot be reviewed", id)
		}
		var changed []string
		if f.Status != status {
			changed = append(changed, "status")
		}
		if f.Rationale != rationale {
			changed = append(changed, "rationale")
		}
		if f.Scope != scope {
			changed = append(changed, "scope")
		}
		f.Status = status
		f.Rationale = rationale
		f.Scope = scope
		f.ChangedAt = now
		f.NeedsReassessment = false
		f.Changes = append(f.Changes, lintBaselineEvent{At: now, Change: "reviewed", Status: status, Fields: changed})
		found = true
		break
	}
	if !found {
		return fmt.Errorf("baseline finding %s not found", id)
	}
	base.Version = ctxLintBaselineVersion
	base.Generated = now
	return saveLintBaseline(base)
}

// writeLintBaseline records the current findings and reports the delta it
// accepted: how many findings are new to the baseline and how many left it.
const (
	lintBaselineUnreviewed         = "unreviewed"
	lintBaselineToFix              = "to fix"
	lintBaselineJustifiedException = "justified exception"
)

func writeLintBaseline(issues []ctxLintIssue, selected map[string]bool) (recorded, added, removed int, err error) {
	previous, err := loadLintBaseline()
	if err != nil {
		return 0, 0, 0, err
	}
	now := contextNow().UTC().Format(time.RFC3339)
	base := lintBaseline{Version: ctxLintBaselineVersion, Generated: contextNow().UTC().Format(time.RFC3339)}
	old := map[string]lintBaselineFinding{}
	if previous != nil {
		for _, f := range previous.Findings {
			old[lintFindingKey(f.Path, f.Priority, f.Message)] = f
		}
	}
	current, seen, count := currentLintBaselineFindings(issues, selected, old, now)
	base.Findings = append(base.Findings, current...)
	recorded, added = count.recorded, count.added
	removedFindings, removed := missingLintBaselineFindings(old, seen, selected, now)
	base.Findings = append(base.Findings, removedFindings...)
	sort.Slice(base.Findings, func(i, j int) bool {
		a, b := base.Findings[i], base.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.Message < b.Message
	})
	if err := saveLintBaseline(&base); err != nil {
		return 0, 0, 0, err
	}
	return recorded, added, removed, nil
}

type lintBaselineWriteCount struct {
	recorded int
	added    int
}

func currentLintBaselineFindings(issues []ctxLintIssue, selected map[string]bool, old map[string]lintBaselineFinding, now string) ([]lintBaselineFinding, map[string]bool, lintBaselineWriteCount) {
	seen := map[string]bool{}
	findings := make([]lintBaselineFinding, 0, len(issues))
	var count lintBaselineWriteCount
	for _, it := range issues {
		if selected != nil && !selected[it.Path] {
			continue
		}
		key := lintFindingKey(it.Path, it.Priority, it.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		count.recorded++
		f, exists := old[key]
		if !exists {
			f = lintBaselineFinding{ID: lintFindingID(it.Path, it.Priority, it.Message), Path: it.Path, Priority: it.Priority, Message: it.Message, Status: lintBaselineUnreviewed, FirstSeen: now, ChangedAt: now}
			f.Changes = []lintBaselineEvent{{At: now, Change: "detected", Status: lintBaselineUnreviewed}}
			count.added++
		} else if !f.Present {
			f.ChangedAt = now
			f.Changes = append(f.Changes, lintBaselineEvent{At: now, Change: "returned", Status: f.Status})
			if f.Status == lintBaselineJustifiedException {
				f.NeedsReassessment = true
				f.Changes = append(f.Changes, lintBaselineEvent{At: now, Change: "reassessment_required", Status: f.Status})
			}
		}
		f.Present = true
		f.LastSeen = now
		f.ResolvedAt = ""
		findings = append(findings, f)
	}
	return findings, seen, count
}

func missingLintBaselineFindings(old map[string]lintBaselineFinding, seen, selected map[string]bool, now string) ([]lintBaselineFinding, int) {
	findings := make([]lintBaselineFinding, 0, len(old))
	removed := 0
	for key, f := range old {
		if seen[key] {
			continue
		}
		if selected != nil && !selected[f.Path] {
			findings = append(findings, f)
			continue
		}
		if f.Present {
			f.Present = false
			f.ResolvedAt = now
			f.ChangedAt = now
			f.Changes = append(f.Changes, lintBaselineEvent{At: now, Change: "resolved", Status: f.Status})
			f = requireExceptionReassessment(f, now)
			removed++
		}
		findings = append(findings, f)
	}
	return findings, removed
}

func requireExceptionReassessment(f lintBaselineFinding, now string) lintBaselineFinding {
	if f.Status != lintBaselineJustifiedException {
		return f
	}
	f.NeedsReassessment = true
	f.Changes = append(f.Changes, lintBaselineEvent{At: now, Change: "reassessment_required", Status: f.Status})
	return f
}

func saveLintBaseline(base *lintBaseline) error {
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sdtWorkDir, 0o750); err != nil { //#nosec G301 -- corpus dir
		return err
	}
	//#nosec G306 -- baseline is a reviewed corpus document
	if err := os.WriteFile(ctxLintBaselineFile, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

type lintBaselineChange struct {
	ID       string   `json:"id" yaml:"id"`
	Path     string   `json:"path" yaml:"path"`
	Priority string   `json:"priority" yaml:"priority"`
	Message  string   `json:"message" yaml:"message"`
	At       string   `json:"at" yaml:"at"`
	Change   string   `json:"change" yaml:"change"`
	Status   string   `json:"status,omitempty" yaml:"status,omitempty"`
	Fields   []string `json:"fields,omitempty" yaml:"fields,omitempty"`
}

type lintBaselineChanges struct {
	Since             string               `json:"since" yaml:"since"`
	HistoryIncomplete bool                 `json:"history_incomplete" yaml:"history_incomplete"`
	Changes           []lintBaselineChange `json:"changes" yaml:"changes"`
}

func lintBaselineChangesSince(spec string) (lintBaselineChanges, error) {
	since, err := time.Parse(time.RFC3339, spec)
	if err != nil {
		return lintBaselineChanges{}, fmt.Errorf("--changes-since must be an RFC3339 timestamp: %w", err)
	}
	base, err := loadLintBaseline()
	if err != nil {
		return lintBaselineChanges{}, err
	}
	if base == nil {
		return lintBaselineChanges{}, fmt.Errorf("%s: no baseline exists", ctxLintBaselineFile)
	}
	if base.Version == 1 {
		return lintBaselineChanges{}, fmt.Errorf("%s: change history is unavailable until v1 is migrated; run context lint --baseline-write", ctxLintBaselineFile)
	}
	result := lintBaselineChanges{Since: spec, Changes: []lintBaselineChange{}}
	for _, f := range base.Findings {
		result.HistoryIncomplete = result.HistoryIncomplete || f.LegacyHistoryUnknown
		for _, event := range f.Changes {
			at, parseErr := time.Parse(time.RFC3339, event.At)
			if parseErr != nil || at.Before(since) {
				continue
			}
			result.Changes = append(result.Changes, lintBaselineChange{ID: f.ID, Path: f.Path, Priority: f.Priority, Message: f.Message, At: event.At, Change: event.Change, Status: event.Status, Fields: append([]string(nil), event.Fields...)})
		}
	}
	sort.SliceStable(result.Changes, func(i, j int) bool {
		if result.Changes[i].At != result.Changes[j].At {
			return result.Changes[i].At < result.Changes[j].At
		}
		if result.Changes[i].Path != result.Changes[j].Path {
			return result.Changes[i].Path < result.Changes[j].Path
		}
		return result.Changes[i].ID < result.Changes[j].ID
	})
	return result, nil
}

// filterAgainstBaseline splits findings into the ones the baseline accepted and
// the ones it has never seen. Accepted findings are returned too, so
// `--suggestions` can re-surface the suggestions among them.
func filterAgainstBaseline(issues []ctxLintIssue, base *lintBaseline) (fresh, accepted []ctxLintIssue) {
	if base == nil {
		return issues, nil
	}
	known := make(map[string]bool, len(base.Findings))
	for _, f := range base.Findings {
		if f.Present && !f.NeedsReassessment {
			known[lintFindingKey(f.Path, f.Priority, f.Message)] = true
		}
	}
	for _, it := range issues {
		if known[lintFindingKey(it.Path, it.Priority, it.Message)] {
			accepted = append(accepted, it)
			continue
		}
		fresh = append(fresh, it)
	}
	return fresh, accepted
}

// baselineAcceptedSuggestions returns the accepted findings worth re-surfacing
// under `--suggestions`: the suggestions, never the warnings or criticals.
func baselineAcceptedSuggestions(accepted []ctxLintIssue) []ctxLintIssue {
	out := make([]ctxLintIssue, 0, len(accepted))
	for _, it := range accepted {
		if it.Priority == ctxLintSuggestion {
			out = append(out, it)
		}
	}
	return out
}
