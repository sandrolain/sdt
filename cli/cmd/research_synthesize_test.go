package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// synthesizedRun returns a project root + runID with one verified source.
func synthesizedRun(t *testing.T) (string, string) {
	t.Helper()
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "why")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	execute(t, researchVerifyCmd, nil, "--run", runID)
	return root, runID
}

func TestResearchSynthesizeToStdout(t *testing.T) {
	root, runID := synthesizedRun(t)
	out := string(execute(t, researchSynthesizeCmd, nil, "--run", runID))
	for _, want := range []string{"# Research draft:", "## Sources", "## Excerpts"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "1 cited source(s)") {
		t.Errorf("expected one cited source:\n%s", out)
	}
	// Nothing was written to disk by a stdout synthesis.
	entries, _ := os.ReadDir(filepath.Join(root, "context", "tmp"))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("stdout synthesis must not write a file: %s", e.Name())
		}
	}
}

func TestResearchSynthesizeWritesTransientTarget(t *testing.T) {
	root, runID := synthesizedRun(t)
	out := filepath.Join("context", "tmp", "draft.md")

	execute(t, researchSynthesizeCmd, nil, "--run", runID, "--out", out)

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(out)))
	if err != nil {
		t.Fatalf("draft not written: %v", err)
	}
	if !strings.Contains(string(data), "## Excerpts") {
		t.Errorf("draft content unexpected:\n%s", data)
	}
}

func TestResearchSynthesizeRefusesNonTransientTarget(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	execute(t, researchVerifyCmd, nil, "--run", runID)

	if code := runSynthesize(t, "--run", runID, "--out", "context/analysis/evil.md"); code == 0 {
		t.Fatal("writing outside context/tmp or *.draft.md must be refused")
	}
}

func TestResearchSynthesizeDraftSuffixAllowed(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	execute(t, researchVerifyCmd, nil, "--run", runID)

	execute(t, researchSynthesizeCmd, nil, "--run", runID, "--out", "notes.draft.md")
	if _, err := os.Stat("notes.draft.md"); err != nil {
		t.Fatalf("a *.draft.md target must be allowed: %v", err)
	}
}

func TestResearchSynthesizeMaxLines(t *testing.T) {
	_, runID := synthesizedRun(t)
	out := string(execute(t, researchSynthesizeCmd, nil, "--run", runID, "--max-lines", "1"))
	if !strings.Contains(out, "truncated") && !strings.Contains(out, "…") {
		t.Errorf("a 1-line cap should truncate the excerpt:\n%s", out)
	}
}

// runSynthesize executes the synthesize command and reports its exit code.
func runSynthesize(t *testing.T, args ...string) (exited int) {
	t.Helper()
	origExit := exit
	exit = func(code int) {
		exited = code
		panic("exit")
	}
	defer func() {
		exit = origExit
		if r := recover(); r != nil && r != "exit" {
			panic(r)
		}
	}()
	execute(t, researchSynthesizeCmd, nil, args...)
	return exited
}
