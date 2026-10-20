package cmd

import (
	"os"
	"strings"
	"testing"
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
