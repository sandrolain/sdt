package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── repo derivation (Phase 4) ─────────────────────────────────────────────────

// writeTestRepoProject writes a synthetic repo layout with an AGENTS.md project
// block, a Taskfile and top-level dirs, to exercise evidence derivation.
func writeTestRepoProject(t *testing.T, dir string) {
	t.Helper()
	agents := `# Repo

## Other

not part of the project block
<!-- sdt:begin:project -->
### Stack
Go 1.27.1 · pure-Go, no CGO · cobra CLI + viper

### Build & Run
` + "```bash" + `
task build
` + "```" + `

### Test
` + "```bash" + `
go test ./...
` + "```" + `

### Lint & Format
golangci-lint run ./cli/...

### Conventions
- Conventional Commits
- no CGO
<!-- sdt:end:project -->
`
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0o600); err != nil {
		t.Fatal(err)
	}
	taskfile := "version: \"3\"\ntasks:\n  build:\n    cmds: [go build ./cli]\n  test:\n    cmds: [go test ./...]\n"
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(taskfile), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"cli", "internal", "web", "test", "docs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeriveRoleProjectFactsRepo(t *testing.T) {
	dir := runInTempDir(t)
	writeTestRepoProject(t, dir)

	f := deriveRoleProjectFacts()
	if f.Stack == "" || !strings.Contains(f.Stack, "Go 1.27.1") {
		t.Errorf("stack not derived from AGENTS.md block: %q", f.Stack)
	}
	if f.Build != "task build" {
		t.Errorf("build must come from AGENTS.md block, got %q", f.Build)
	}
	if f.Test != "go test ./..." {
		t.Errorf("test not derived, got %q", f.Test)
	}
	if f.Lint == "" || !strings.Contains(f.Lint, "golangci-lint") {
		t.Errorf("lint not derived, got %q", f.Lint)
	}
	if len(f.Conventions) != 2 {
		t.Errorf("expected 2 conventions, got %q", f.Conventions)
	}
	for _, want := range []string{"cli", "internal", "web", "test", "docs"} {
		if !containsString(f.Layout, want) {
			t.Errorf("layout missing %q: %v", want, f.Layout)
		}
	}
	if !containsString(f.OwnedPaths["backend"], "cli") || !containsString(f.OwnedPaths["backend"], "internal") {
		t.Errorf("backend owned paths not derived: %v", f.OwnedPaths["backend"])
	}
	if !containsString(f.OwnedPaths["frontend"], "web") {
		t.Errorf("frontend owned paths not derived: %v", f.OwnedPaths["frontend"])
	}
	if len(f.Assumptions) != 0 {
		t.Errorf("fully derived repo must have no assumptions, got %q", f.Assumptions)
	}
}

func TestDeriveRoleProjectFactsNoEvidence(t *testing.T) {
	runInTempDir(t)

	f := deriveRoleProjectFacts()
	if f.Stack != "" || f.Build != "" || f.Test != "" || f.Lint != "" {
		t.Errorf("no-evidence repo must derive nothing: %+v", f)
	}
	if len(f.Assumptions) != 4 {
		t.Errorf("expected 4 assumptions (stack/build/test/lint), got %q", f.Assumptions)
	}
	if len(f.OwnedPaths["frontend"]) != 0 {
		t.Errorf("no directed evidence must not invent owned paths: %v", f.OwnedPaths)
	}
}

func TestDeriveRoleProjectFactsNeverInvent(t *testing.T) {
	runInTempDir(t)
	// AGENTS.md block present but only fill-in comments: sections stay empty.
	agents := "# R\n`<!-- sdt:begin:project -->`\n### Stack\n<!-- fill in -->\n`<!-- sdt:end:project -->`\n"
	if err := os.WriteFile("AGENTS.md", []byte(agents), 0o600); err != nil {
		t.Fatal(err)
	}
	f := deriveRoleProjectFacts()
	if f.Stack != "" {
		t.Errorf("comment-only stack must not be invented, got %q", f.Stack)
	}
	if !strings.Contains(strings.Join(f.Assumptions, " "), "stack") {
		t.Errorf("stack must appear in assumptions: %q", f.Assumptions)
	}
}

func TestTaskCommandInference(t *testing.T) {
	runInTempDir(t)
	tf := "version: \"3\"\ntasks:\n  lint:\n    cmds: [golangci-lint run]\n"
	if err := os.WriteFile("Taskfile.yml", []byte(tf), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, ok := taskfileCommand("lint"); !ok || c != "task lint" {
		t.Errorf("taskfile lint not inferred: %q %v", c, ok)
	}
	if _, ok := taskfileCommand("build"); ok {
		t.Error("taskfile build must be absent")
	}
}

// ── project layer rendering + assumption footer ────────────────────────────────

// containsString reports whether s contains v.
func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestRoleProjectLayerAssumptionFooter(t *testing.T) {
	runInTempDir(t)
	r := roleDescriptor{Slug: "backend", Title: "Backend engineer"}
	body := roleProjectLayer(r, deriveRoleProjectFacts())

	for _, want := range []string{
		"not derivable (assumption)",
		"### Open assumptions",
		"never left as silent rules",
		"context/instructions/project.md",
		"sdt agent roles init --force",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("project layer missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "task build") || strings.Contains(body, "repo layout:") {
		t.Errorf("no-evidence layer must not invent facts:\n%s", body)
	}
}

func TestRoleProjectLayerRepoTags(t *testing.T) {
	dir := runInTempDir(t)
	writeTestRepoProject(t, dir)
	r := roleDescriptor{Slug: "backend", Title: "Backend engineer"}
	body := roleProjectLayer(r, deriveRoleProjectFacts())

	if !strings.Contains(body, "(repo)") {
		t.Errorf("derived layer must tag lines (repo):\n%s", body)
	}
	if strings.Contains(body, "### Open assumptions") {
		t.Errorf("fully derived layer must not carry an assumption footer:\n%s", body)
	}
	if !strings.Contains(body, "Owned repo paths") {
		t.Errorf("backend layer must list owned repo paths from layout:\n%s", body)
	}
	if strings.Count(body, "(assumption)") != 1 {
		t.Errorf("no assumptions expected in the fully derived layer (only the legend tag):\n%s", body)
	}
}

// ── questionnaire (≤3 non-derivable role facts) ────────────────────────────────

func TestApplyRoleAnswersUserTagging(t *testing.T) {
	base := deriveRoleProjectFacts()
	f := applyRoleAnswers(base, []string{"reviewer + qa", "", "semver releases"})
	if len(f.UserAnswers) != 2 {
		t.Fatalf("expected 2 user answers, got %q", f.UserAnswers)
	}
	// Empty answers are dropped; kept answers carry "label → answer" prose.
	for _, a := range f.UserAnswers {
		if !strings.Contains(a, "→") {
			t.Errorf("answer should carry the question label: %q", a)
		}
	}
}

func TestApplyRoleAnswersBounds(t *testing.T) {
	f := deriveRoleProjectFacts()
	long := make([]string, 6)
	for i := range long {
		long[i] = "extra answer"
	}
	got := applyRoleAnswers(f, long)
	want := len(roleQuestionnaireItems)
	if got := len(got.UserAnswers); got != want {
		t.Errorf("questionnaire must accept at most %d answers, got %d", want, got)
	}
}

func TestRoleProjectLayerUserAnswers(t *testing.T) {
	runInTempDir(t)
	r := roleDescriptor{Slug: "reviewer", Title: "Reviewer"}
	facts := applyRoleAnswers(deriveRoleProjectFacts(), []string{"no self-review"})
	body := roleProjectLayer(r, facts)
	if !strings.Contains(body, "no self-review") {
		t.Errorf("user answer missing from layer:\n%s", body)
	}
	if !strings.Contains(body, "(user)") {
		t.Errorf("user answers must be tagged (user):\n%s", body)
	}
}

func TestRunRoleQuestionnaireNonInteractive(t *testing.T) {
	// Non-interactive stdin: --ask must not hang or collect answers.
	runInTempDir(t)
	out := execute(t, agentRolesInitCmd, nil, "--project", "p", "--ask")
	if !strings.Contains(string(out), "[created]") {
		t.Fatalf("--ask run must still generate profiles:\n%s", out)
	}
	data, err := os.ReadFile(filepath.Join(sdtRolesDir, "pm.md"))
	if err != nil {
		t.Fatal(err)
	}
	// TF enable: non-interactive --ask must not embed questionnaire answers even
	// when prompts print under a TTY; answers arrive as "label → answer".
	for _, marker := range []string{"Review pairing", "Deployment/release"} {
		if strings.Contains(string(data), marker) {
			t.Errorf("non-interactive --ask must not add user answers:\n%s", data)
		}
	}
}
