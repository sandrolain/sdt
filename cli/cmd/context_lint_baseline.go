package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
const ctxLintBaselineVersion = 1

type lintBaseline struct {
	Version   int                   `json:"version"`
	Generated string                `json:"generated"`
	Findings  []lintBaselineFinding `json:"findings"`
}

type lintBaselineFinding struct {
	Path     string `json:"path"`
	Priority string `json:"priority"`
	Message  string `json:"message"`
}

// lintFindingKey is the identity of a finding: the same document, severity and
// message is the same defect. A message that changes is a new finding on
// purpose — rewording the check should surface the reworded result.
func lintFindingKey(path, priority, message string) string {
	return path + "\x00" + priority + "\x00" + message
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
	if base.Version != ctxLintBaselineVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (this build writes %d)", ctxLintBaselineFile, base.Version, ctxLintBaselineVersion)
	}
	return &base, nil
}

// writeLintBaseline records the current findings and reports the delta it
// accepted: how many findings are new to the baseline and how many left it.
func writeLintBaseline(issues []ctxLintIssue) (added, removed int, err error) {
	previous, err := loadLintBaseline()
	if err != nil {
		return 0, 0, err
	}
	seen := map[string]bool{}
	base := lintBaseline{Version: ctxLintBaselineVersion, Generated: contextNow().UTC().Format(time.RFC3339)}
	for _, it := range issues {
		key := lintFindingKey(it.Path, it.Priority, it.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		base.Findings = append(base.Findings, lintBaselineFinding{Path: it.Path, Priority: it.Priority, Message: it.Message})
	}
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
	if previous != nil {
		old := map[string]bool{}
		for _, f := range previous.Findings {
			old[lintFindingKey(f.Path, f.Priority, f.Message)] = true
		}
		for _, f := range base.Findings {
			if !old[lintFindingKey(f.Path, f.Priority, f.Message)] {
				added++
			}
		}
		for _, f := range previous.Findings {
			if !seen[lintFindingKey(f.Path, f.Priority, f.Message)] {
				removed++
			}
		}
	}
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return 0, 0, err
	}
	if err := os.MkdirAll(sdtWorkDir, 0o750); err != nil { //#nosec G301 -- corpus dir
		return 0, 0, err
	}
	//#nosec G306 -- baseline is a reviewed corpus document
	if err := os.WriteFile(ctxLintBaselineFile, append(data, '\n'), 0o644); err != nil {
		return 0, 0, err
	}
	return added, removed, nil
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
		known[lintFindingKey(f.Path, f.Priority, f.Message)] = true
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
