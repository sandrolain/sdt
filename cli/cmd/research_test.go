package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

// researchTestProject creates a temp project root with a .sdt.yaml, chdirs into
// it and returns the root.
func researchTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, sdtConfigFile), []byte("project: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return root
}

func TestResearchInitCreatesRun(t *testing.T) {
	root := researchTestProject(t)

	out := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "vector stores", "--scope", "embedded")))
	if out == "" {
		t.Fatal("init did not print a run id")
	}

	// The manifest and sidecar exist on disk, with the query recorded.
	manifest := research.ManifestPath(root, out)
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	r, err := research.Load(root, out)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.Query != "vector stores" || r.Scope != "embedded" {
		t.Errorf("run = %+v", r)
	}
	if !strings.Contains(string(data), "run_id:") {
		t.Errorf("manifest missing run_id:\n%s", data)
	}
}

func TestResearchInitObjectiveFlag(t *testing.T) {
	root := researchTestProject(t)
	out := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "capture layout", "--objective", "web-capture-tooling")))
	r, err := research.Load(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if r.Objective != "web-capture-tooling" {
		t.Errorf("objective = %q, want web-capture-tooling", r.Objective)
	}

	inspect := string(execute(t, researchInspectCmd, nil))
	if !strings.Contains(inspect, "objective: web-capture-tooling") {
		t.Errorf("inspect must show the objective:\n%s", inspect)
	}
}

func TestResearchInitDerivesObjectiveFromQuery(t *testing.T) {
	root := researchTestProject(t)
	out := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "clean architecture notes")))
	r, err := research.Load(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if r.Objective != "clean-architecture-notes" {
		t.Errorf("derived objective = %q, want clean-architecture-notes", r.Objective)
	}
}

func TestResearchInitObjectiveBadSlug(t *testing.T) {
	researchTestProject(t)
	if code := runResearchInitExit(t, "--query", "q", "--objective", "Not A Slug"); code != 1 {
		t.Fatalf("a non-kebab-case objective must exit 1, got %d", code)
	}
}

// runResearchInitExit executes research init and reports the exit code it
// triggered, recovering from the test exit override.
func runResearchInitExit(t *testing.T, args ...string) (exited int) {
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
	execute(t, researchInitCmd, nil, args...)
	return exited
}

func TestResearchInspectLatestText(t *testing.T) {
	root := researchTestProject(t)
	out := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q1")))

	got := string(execute(t, researchInspectCmd, nil))
	if !strings.Contains(got, out) {
		t.Errorf("inspect did not show the latest run %s:\n%s", out, got)
	}
	if !strings.Contains(got, "query: q1") {
		t.Errorf("inspect missing the query:\n%s", got)
	}
	_ = root
}

func TestResearchInspectJSON(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "json query")))

	raw := string(execute(t, researchInspectCmd, nil, "--format", "json"))
	var view struct {
		RunID   string           `json:"run_id"`
		Query   string           `json:"query"`
		Sources []map[string]any `json:"sources"`
	}
	if err := json.Unmarshal([]byte(raw), &view); err != nil {
		t.Fatalf("inspect --format json is not valid JSON: %v\n%s", err, raw)
	}
	if view.RunID != runID || view.Query != "json query" {
		t.Errorf("view = %+v", view)
	}
	if view.Sources == nil {
		t.Errorf("sources must serialize as [] not null:\n%s", raw)
	}
}

func TestResearchInspectNoRun(t *testing.T) {
	researchTestProject(t)
	// execute() fails the test on a non-zero exit, so call the resolver directly.
	if _, _, err := resolveRun(researchInspectCmd); err == nil {
		t.Fatal("expected an error when no run exists")
	}
}

func TestResearchRootWalksUp(t *testing.T) {
	root := researchTestProject(t)
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	got, err := researchRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("researchRoot = %q, want %q", got, root)
	}
}
