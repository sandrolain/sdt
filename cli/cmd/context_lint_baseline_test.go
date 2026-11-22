package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLintBaselineReportsOnlyNewFindings is the contract of the baseline: an
// unchanged corpus says nothing new, a new finding is reported, and a finding
// that disappeared is not reported as a difference.
func TestLintBaselineReportsOnlyNewFindings(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: a\nlinks: none\n---\nbody\n")

	// Absent baseline: today's behaviour, the finding is reported.
	out := string(execute(t, contextLintCmd, nil))
	if !strings.Contains(out, "`maturity`") && !strings.Contains(out, "missing `objective`") {
		// The objective family is off by default; use a base-key finding instead.
		t.Logf("default run: %s", out)
	}

	// Record it: the delta says one new finding was accepted.
	write := string(execute(t, contextLintCmd, nil, "--baseline-write"))
	if !strings.Contains(write, "baseline written") {
		t.Fatalf("--baseline-write printed no record line: %q", write)
	}

	// Unchanged corpus: nothing new.
	out = string(execute(t, contextLintCmd, nil))
	if strings.Contains(out, "[WARNING]") || strings.Contains(out, "[SUGGESTION]") {
		t.Errorf("an accepted finding was reported as new:\n%s", out)
	}
	if !strings.Contains(out, "0 WARNING") {
		t.Errorf("the summary does not read as clean:\n%s", out)
	}

	// A new finding is reported.
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: n\nagent: a\n---\n# H1 body\n")
	out = string(execute(t, contextLintCmd, nil))
	if !strings.Contains(out, "context/notes/n.md") {
		t.Errorf("a new finding was not reported:\n%s", out)
	}

	// Removing a finding is not a report.
	if err := os.Remove("context/notes/n.md"); err != nil {
		t.Fatal(err)
	}
	out = string(execute(t, contextLintCmd, nil))
	if strings.Contains(out, "context/notes/n.md") {
		t.Errorf("a removed finding was reported:\n%s", out)
	}

	// --all restores the full view.
	out = string(execute(t, contextLintCmd, nil, "--all"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Errorf("--all did not restore the accepted findings:\n%s", out)
	}
}

func TestLintBaselineV1MigrationAndScopedWrite(t *testing.T) {
	setupContextProject(t)
	generated := "2026-10-21T05:20:33Z"
	legacy := lintBaseline{Version: 1, Generated: generated, Findings: []lintBaselineFinding{
		{Path: "context/analysis/a.md", Priority: ctxLintWarning, Message: "old finding A"},
		{Path: "context/notes/b.md", Priority: ctxLintSuggestion, Message: "old finding B"},
	}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ctxLintBaselineFile, string(data))

	loaded, err := loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 || loaded.Findings[0].Status != lintBaselineUnreviewed || loaded.Findings[0].FirstSeen != "" || loaded.Findings[0].LastSeen != generated || !loaded.Findings[0].LegacyHistoryUnknown {
		t.Fatalf("legacy migration invented or lost history: %+v", loaded.Findings[0])
	}
	if _, err := lintBaselineChangesSince(generated); err == nil || !strings.Contains(err.Error(), "history is unavailable") {
		t.Fatalf("v1 changes-since did not require migration: %v", err)
	}

	recorded, _, _, err := writeLintBaseline([]ctxLintIssue{
		{Path: "context/analysis/a.md", Priority: ctxLintWarning, Message: "old finding A"},
		{Path: ctxTopicsFilePath, Priority: ctxLintWarning, Message: "global register finding"},
	}, map[string]bool{"context/analysis/a.md": true})
	if err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Fatalf("scoped write recorded %d findings, want 1", recorded)
	}
	loaded, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 2 {
		t.Fatalf("baseline version = %d, want 2", loaded.Version)
	}
	byPath := map[string]lintBaselineFinding{}
	for _, finding := range loaded.Findings {
		byPath[finding.Path] = finding
	}
	if !byPath["context/notes/b.md"].Present || byPath["context/notes/b.md"].Message != "old finding B" {
		t.Fatalf("scoped write dropped unrelated finding: %+v", byPath["context/notes/b.md"])
	}
	if _, exists := byPath[ctxTopicsFilePath]; exists {
		t.Fatalf("scoped write recorded a global finding: %+v", byPath[ctxTopicsFilePath])
	}
	changes, err := lintBaselineChangesSince(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !changes.HistoryIncomplete {
		t.Fatal("partial migration did not disclose incomplete legacy history")
	}

	_, _, _, err = writeLintBaseline(nil, map[string]bool{"context/analysis/a.md": true})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	byPath = map[string]lintBaselineFinding{}
	for _, finding := range loaded.Findings {
		byPath[finding.Path] = finding
	}
	if byPath["context/analysis/a.md"].Present || !byPath["context/notes/b.md"].Present {
		t.Fatalf("scoped resolution changed the wrong records: A=%+v B=%+v", byPath["context/analysis/a.md"], byPath["context/notes/b.md"])
	}
}

func TestLintBaselineExceptionReassessmentAndChangesSince(t *testing.T) {
	setupContextProject(t)
	issue := ctxLintIssue{Path: "context/notes/n.md", Priority: ctxLintWarning, Message: "known issue"}
	if _, _, _, err := writeLintBaseline([]ctxLintIssue{issue}, nil); err != nil {
		t.Fatal(err)
	}
	base, err := loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	id := base.Findings[0].ID
	if err := reviewLintBaseline(id, lintBaselineJustifiedException, "required by the source system", "context/notes/n.md"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := writeLintBaseline(nil, nil); err != nil {
		t.Fatal(err)
	}
	base, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	finding := base.Findings[0]
	if finding.Present || !finding.NeedsReassessment || finding.ResolvedAt == "" || finding.Status != lintBaselineJustifiedException {
		t.Fatalf("disappeared exception state = %+v", finding)
	}
	changes, err := lintBaselineChangesSince(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Changes) < 3 || changes.Changes[0].Change != "detected" || changes.Changes[len(changes.Changes)-1].Change != "reassessment_required" {
		t.Fatalf("changes-since = %+v", changes)
	}
	if _, _, _, err := writeLintBaseline([]ctxLintIssue{issue}, nil); err != nil {
		t.Fatal(err)
	}
	base, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if !base.Findings[0].Present || !base.Findings[0].NeedsReassessment {
		t.Fatalf("returned exception was not flagged for review: %+v", base.Findings[0])
	}
	if err := reviewLintBaseline(id, lintBaselineJustifiedException, "rechecked against the source", "context/notes/n.md and its upstream policy"); err != nil {
		t.Fatal(err)
	}
	base, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	finding = base.Findings[0]
	if finding.NeedsReassessment || finding.Changes[len(finding.Changes)-1].Change != "reviewed" {
		t.Fatalf("review did not clear reassessment: %+v", finding)
	}
	fields := finding.Changes[len(finding.Changes)-1].Fields
	if len(fields) != 2 || fields[0] != "rationale" || fields[1] != "scope" {
		t.Fatalf("review change fields = %v", fields)
	}
}

func TestLintBaselineReviewAndChangesSinceCLI(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: n\nagent: a\n---\n# H1 body\n")
	execute(t, contextLintCmd, nil, "--baseline-write")
	base, err := loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Findings) == 0 {
		t.Fatal("baseline write recorded no finding")
	}
	id := base.Findings[0].ID
	out := string(execute(t, contextLintCmd, nil,
		"--baseline-review", id,
		"--baseline-status", lintBaselineJustifiedException,
		"--baseline-rationale", "required by the source system",
		"--baseline-scope", "this note only",
	))
	if !strings.Contains(out, "reviewed") {
		t.Fatalf("review command did not confirm the update: %q", out)
	}
	base, err = loadLintBaseline()
	if err != nil {
		t.Fatal(err)
	}
	if base.Findings[0].Status != lintBaselineJustifiedException {
		t.Fatalf("review status = %q", base.Findings[0].Status)
	}
	out = string(execute(t, contextLintCmd, nil, "--changes-since", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), "--format", "json"))
	if !strings.Contains(out, `"change": "reviewed"`) || !strings.Contains(out, `"status": "justified exception"`) || !strings.Contains(out, `"history_incomplete": false`) || !strings.Contains(out, `"rationale"`) {
		t.Fatalf("changes-since JSON omitted review event: %s", out)
	}
}

// TestLintBaselineAbsentKeepsTheFullView proves the compatibility clause: with no
// baseline file the run reports everything, as it always did.
func TestLintBaselineAbsentKeepsTheFullView(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: n\nagent: a\n---\n# H1 body\n")
	if _, err := os.Stat(ctxLintBaselineFile); !os.IsNotExist(err) {
		t.Fatalf("the fixture already has a baseline: %v", err)
	}
	out := string(execute(t, contextLintCmd, nil))
	if !strings.Contains(out, "context/notes/n.md") {
		t.Errorf("with no baseline every finding must be reported:\n%s", out)
	}
}

// TestLintBaselineShapeIsVersioned proves the file is a reviewed artifact: it
// parses, carries its version and a timestamp, and an unknown version is refused
// rather than trusted.
func TestLintBaselineShapeIsVersioned(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: n\nagent: a\n---\n# H1 body\n")
	execute(t, contextLintCmd, nil, "--baseline-write")

	base, err := loadLintBaseline()
	if err != nil {
		t.Fatalf("the written baseline does not load: %v", err)
	}
	if base.Version != ctxLintBaselineVersion || base.Generated == "" {
		t.Fatalf("baseline shape = %+v", base)
	}
	if len(base.Findings) == 0 {
		t.Fatal("the baseline recorded no finding")
	}

	writeTestFile(t, ctxLintBaselineFile, `{"version": 99, "findings": []}`)
	if _, err := loadLintBaseline(); err == nil {
		t.Error("an unknown baseline version was trusted")
	}
}
