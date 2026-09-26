package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

// stubContextNow pins contextNow to a fixed timestamp for deterministic paths
// and restores it when the test finishes.
func stubContextNow(t *testing.T, fixed time.Time) {
	t.Helper()
	orig := contextNow
	contextNow = func() time.Time { return fixed }
	t.Cleanup(func() { contextNow = orig })
}

func TestContextPath(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextPathCmd, nil, "--type", "worklog", "--slug", "Review Deps!")
	want := filepath.Join("context", "worklog", "20260806-070000-review-deps.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "plan")
	want = filepath.Join("context", "plan", "20260806-070000.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "analysis", "--slug", "backend-choice")
	want = filepath.Join("context", "analysis", "20260806-070000-backend-choice.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "proposal", "--slug", "prompt-provenance")
	want = filepath.Join("context", "proposals", "20260806-070000-prompt-provenance.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "prompt", "--slug", "deepsearch")
	want = filepath.Join("context", "prompts", "20260806-070000-deepsearch.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestContextPathJSON(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextPathCmd, nil, "--type", "plan", "--format", "json")
	var res contextPathResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if res.Type != "plan" {
		t.Errorf("unexpected type: %s", res.Type)
	}
	if !strings.HasSuffix(res.Path, "plan/20260806-070000.md") {
		t.Errorf("unexpected path: %s", res.Path)
	}
}

func TestContextPathMissingType(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextPathCmd, nil))
	})
}

func TestContextPathUnknownType(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextPathCmd, nil, "--type", "bogus"))
	})
}

func TestContextNew(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "worklog", "--slug", "review-deps", "--context", "reviewed deps", "--input", "reviewed all deps")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "worklog", "20260806-070000-review-deps.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected worklog file created: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"kind: worklog",
		"created: 2026-08-06T07:00:00Z",
		"context: reviewed deps",
		"reviewed all deps",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in file:\n%s", want, content)
		}
	}
}

func TestContextNewProposalAndPrompt(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "proposal", "--title", "prompt provenance")
	proposalPath := filepath.Join(dir, "context", "proposals", "20260806-070000-prompt-provenance.md")
	proposal, err := os.ReadFile(proposalPath)
	if err != nil {
		t.Fatalf("expected proposal file: %v", err)
	}
	for _, want := range []string{"kind: proposal", "title: prompt provenance", "status: draft", "## Proposed design", "## Decision outcome"} {
		if !strings.Contains(string(proposal), want) {
			t.Errorf("expected proposal content %q:\n%s", want, proposal)
		}
	}

	// Unknown/removed type names are rejected on creation.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "rfc", "--title", "unknown attempt"))
	})

	execute(t, contextNewCmd, nil, "--type", "prompt", "--title", "deepsearch")
	promptPath := filepath.Join(dir, "context", "prompts", "20260806-070000-deepsearch.md")
	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("expected prompt file: %v", err)
	}
	for _, want := range []string{"kind: prompt", "title: deepsearch", "## Prompt", "## Runs", "| Model/tool |"} {
		if !strings.Contains(string(prompt), want) {
			t.Errorf("expected prompt content %q:\n%s", want, prompt)
		}
	}
}

func TestContextNewResearch(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "research", "--title", "vector backends")
	researchPath := filepath.Join(dir, "context", "research", "20260806-070000-vector-backends.md")
	research, err := os.ReadFile(researchPath)
	if err != nil {
		t.Fatalf("expected research file: %v", err)
	}
	for _, want := range []string{
		"kind: research",
		"title: vector backends",
		"subject: ",
		"status: draft",
		"created: 2026-08-06T07:00:00Z",
		"updated: 2026-08-06T07:00:00Z",
		"## Subject",
		"## Findings",
		"## Evidence",
		"## Feeds",
	} {
		if !strings.Contains(string(research), want) {
			t.Errorf("expected research content %q:\n%s", want, research)
		}
	}

	out := execute(t, contextPathCmd, nil, "--type", "research", "--slug", "vector-backends")
	want := filepath.Join("context", "research", "20260806-070000-vector-backends.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("context path research = %q, want %q", got, want)
	}
}

func TestContextNewPlan(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "ship-memory", "--input", "body")
	data, err := os.ReadFile(filepath.Join(dir, "context", "plan", "20260806-070000-ship-memory.md"))
	if err != nil {
		t.Fatalf("expected plan created: %v", err)
	}
	if !strings.Contains(string(data), "kind: plan") {
		t.Errorf("expected kind: plan:\n%s", data)
	}
}

func TestContextNewQuotesContextWithColon(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "worklog", "--slug", "colon-context", "--context", "Delta here: nested maps", "--input", "body")
	path := strings.TrimSpace(string(out))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected worklog file created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `context: "Delta here: nested maps"`) {
		t.Errorf("expected quoted context to avoid invalid YAML:\n%s", content)
	}
	fmBlock := strings.TrimSpace(strings.SplitN(content, "---", 3)[1])
	var fm struct {
		Kind    string `yaml:"kind"`
		Created string `yaml:"created"`
		Context string `yaml:"context"`
		Project string `yaml:"project"`
	}
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		t.Fatalf("frontmatter must parse as YAML: %v\n%s", err, fmBlock)
	}
	if fm.Kind != "worklog" || fm.Context != "Delta here: nested maps" {
		t.Errorf("unexpected frontmatter values: %+v", fm)
	}
}

func TestContextNewAnalysis(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "backend-choice", "--input", "body")
	data, err := os.ReadFile(filepath.Join(dir, "context", "analysis", "20260806-070000-backend-choice.md"))
	if err != nil {
		t.Fatalf("expected analysis created: %v", err)
	}
	for _, want := range []string{"kind: analysis", "body"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in file:\n%s", want, data)
		}
	}
}

func TestContextNewExistingNoForce(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x", "--input", "first")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x", "--input", "second"))
	})
}

func TestContextNewForce(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x", "--input", "first")
	out := execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x", "--input", "second", "--force")
	if !strings.Contains(string(out), "notes/20260806-070000-x.md") {
		t.Errorf("unexpected output: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "context", "notes", "20260806-070000-x.md"))
	if !strings.Contains(string(data), "second") {
		t.Errorf("expected overwritten content:\n%s", data)
	}
}

func TestContextNewJSON(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "worklog", "--slug", "x", "--format", "json")
	var res contextNewResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if res.Type != "worklog" || res.Status != statusCreated || res.Created == "" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestContextNewInvalidType(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "tasks"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "bogus"))
	})
}

func TestContextNewTitleDerivesSlug(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "worklog", "--title", "Review Deps!")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "worklog", "20260806-070000-review-deps.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
	content := mustReadFile(t, path)
	if !strings.Contains(content, "summary: "+ctxSummaryPlaceholder) {
		t.Errorf("expected placeholder summary, got:\n%s", content)
	}
}

func TestContextNewSummaryFlag(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x",
		"--summary", "A real summary")
	content := mustReadFile(t, filepath.Join("context", "notes", "20260806-070000-x.md"))
	if !strings.Contains(content, "summary: A real summary") {
		t.Errorf("expected --summary value, got:\n%s", content)
	}
}

func TestContextNewFullFrontmatter(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, filepath.Join("context", "plan", "20260912-000000-pipeline.md"),
		"---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")

	cases := []struct {
		typ    string
		flags  []string
		want   []string
		forbid []string
	}{
		{typ: "plan", want: []string{"kind: plan", "status: active", "created: 2026-08-06T07:00:00Z", "updated: 2026-08-06T07:00:00Z"}, forbid: []string{"created_at:"}},
		{typ: "analysis", flags: []string{"--title", "Backend Choice", "--objective", "memory"}, want: []string{"kind: analysis", "title: Backend Choice", "objective: memory", "status: active", "updated: 2026-08-06T07:00:00Z"}, forbid: []string{"created_at:"}},
		{typ: "worklog", want: []string{"kind: worklog", "updated: 2026-08-06T07:00:00Z"}, forbid: []string{"created_at:", "status:"}},
		{typ: "notes", want: []string{"kind: notes", "created: 2026-08-06T07:00:00Z"}, forbid: []string{"created_at:", "status:", "updated:"}},
		{typ: "questions", want: []string{"kind: questions", "status: active", "updated: 2026-08-06T07:00:00Z"}, forbid: []string{"created_at:"}},
	}
	for _, c := range cases {
		args := append([]string{"--type", c.typ, "--slug", "x"}, c.flags...)
		execute(t, contextNewCmd, nil, args...)
		path := filepath.Join(dir, "context", c.typ, "20260806-070000-x.md")
		content := mustReadFile(t, path)
		for _, w := range c.want {
			if !strings.Contains(content, w) {
				t.Errorf("[%s] expected %q in frontmatter:\n%s", c.typ, w, content)
			}
		}
		for _, f := range c.forbid {
			if strings.Contains(content, f) {
				t.Errorf("[%s] did not expect %q:\n%s", c.typ, f, content)
			}
		}
		if !strings.Contains(content, "summary: "+ctxSummaryPlaceholder) {
			t.Errorf("[%s] expected placeholder summary:\n%s", c.typ, content)
		}
	}
}

func TestContextNewObjectiveFlag(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "x",
		"--objective", "memory-backend")
	content := mustReadFile(t, filepath.Join("context", "analysis", "20260806-070000-x.md"))
	if !strings.Contains(content, "objective: memory-backend") {
		t.Errorf("expected objective line:\n%s", content)
	}
}

func TestContextNewObjectiveNotAnalysis(t *testing.T) {
	runInTempDir(t)
	// notes now accept --objective (dead-end grouping); a type that does not
	// support it must still be rejected.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "questions", "--slug", "x", "--objective", "viewer"))
	})
}

func TestContextNewObjectiveBadSlug(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "x", "--objective", "Memory Backend"))
	})
}

func TestContextNewPlanObjective(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "ship-memory", "--objective", "memory-backend")
	content := mustReadFile(t, filepath.Join("context", "plan", "20260806-070000-ship-memory.md"))
	if !strings.Contains(content, "objective: memory-backend") {
		t.Errorf("expected objective line on plan:\n%s", content)
	}
}

func TestContextNewPlanObjectiveFromSource(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "choice", "--objective", "memory-backend")
	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "ship-memory",
		"--source", "analysis/20260806-070000-choice.md")
	content := mustReadFile(t, filepath.Join("context", "plan", "20260806-070000-ship-memory.md"))
	for _, want := range []string{
		"objective: memory-backend",
		"links:\n  - analysis/20260806-070000-choice.md",
		"sources:\n  - analysis/20260806-070000-choice.md",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in plan:\n%s", want, content)
		}
	}
}

func TestContextNewSourceWritesReferences(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "p",
		"--source", "analysis/a.md", "--source", "analysis/b.md", "--objective", "obj")
	content := mustReadFile(t, filepath.Join("context", "plan", "20260806-070000-p.md"))
	for _, want := range []string{
		"links:\n  - analysis/a.md\n  - analysis/b.md",
		"sources:\n  - analysis/a.md\n  - analysis/b.md",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in plan:\n%s", want, content)
		}
	}
}

func TestContextNewArchitecture(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "architecture", "--title", "Config Loading", "--input", "body")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "architecture", "config-loading.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
	content := mustReadFile(t, path)
	for _, w := range []string{
		"kind: architecture",
		"component: config-loading",
		"status: draft",
		"created: 2026-08-06T07:00:00Z",
		"updated: 2026-08-06T07:00:00Z",
	} {
		if !strings.Contains(content, w) {
			t.Errorf("expected %q in file:\n%s", w, content)
		}
	}
}

func TestContextNewArchitectureRequiresSlug(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "architecture"))
	})
}

func TestNextDecisionNumber(t *testing.T) {
	dir := runInTempDir(t)
	dec := filepath.Join(dir, "context", "decisions")

	// Empty / missing dir → 0001
	n, err := nextDecisionNumber()
	if err != nil {
		t.Fatal(err)
	}
	if n != "0001" {
		t.Errorf("empty dir: got %q, want 0001", n)
	}

	os.MkdirAll(dec, 0o750)
	writeCtxDoc(t, filepath.Join(dec, "0002-auth.md"), "---\nkind: decision\nsummary: s\n---\n")
	writeCtxDoc(t, filepath.Join(dec, "0004-temp.md"), "---\nkind: decision\nsummary: s\n---\n")
	writeCtxDoc(t, filepath.Join(dec, "README.md"), "not a decision\n")

	n, err = nextDecisionNumber()
	if err != nil {
		t.Fatal(err)
	}
	if n != "0005" {
		t.Errorf("max=4 → got %q, want 0005", n)
	}
}

func TestContextNewDecision(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "decision", "--title", "Auth choice", "--summary", "Use JWT for auth")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "decisions", "0001-auth-choice.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
	content := mustReadFile(t, path)
	for _, w := range []string{
		"kind: decision",
		"number: 0001",
		"title: Auth choice",
		"summary: Use JWT for auth",
		"status: proposed",
		"created: 2026-08-06T07:00:00Z",
		"links:",
	} {
		if !strings.Contains(content, w) {
			t.Errorf("expected %q in file:\n%s", w, content)
		}
	}
}

func TestContextNewDecisionAutoNumber(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, filepath.Join("context", "decisions", "0003-legacy.md"), "---\nkind: decision\nsummary: l\n---\n")

	out := execute(t, contextNewCmd, nil, "--type", "decision", "--title", "Second", "--slug", "second-choice")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "decisions", "0004-second-choice.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
	if num := mustReadFile(t, path); !strings.Contains(num, "number: 0004") {
		t.Errorf("expected number 0004:\n%s", num)
	}
}

func TestContextNewDecisionNumberOverride(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextNewCmd, nil, "--type", "decision", "--title", "M", "--slug", "m", "--number", "0007")
	path := strings.TrimSpace(string(out))
	if !strings.HasSuffix(path, "0007-m.md") {
		t.Errorf("expected 0007-m.md, got %q", path)
	}
}

func TestContextNewDecisionRequiresSlug(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "decision"))
	})
}

func TestContextNewDecisionBadNumber(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "decision", "--title", "X", "--slug", "x", "--number", "7"))
	})
}

func TestContextPathDecision(t *testing.T) {
	runInTempDir(t)
	out := execute(t, contextPathCmd, nil, "--type", "decision", "--number", "0009", "--slug", "choice")
	path := strings.TrimSpace(string(out))
	want := filepath.Join("context", "decisions", "0009-choice.md")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
}

func TestContextPathDecisionRequiresNumber(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextPathCmd, nil, "--type", "decision", "--slug", "x"))
	})
}

func TestContextPathDecisionBadNumber(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextPathCmd, nil, "--type", "decision", "--number", "42", "--slug", "x"))
	})
}

func TestContextNewLintClean(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	for _, typ := range []string{"plan", "analysis", "worklog", "notes", "questions", "architecture"} {
		execute(t, contextNewCmd, nil, "--type", typ, "--title", "Lint Clean "+typ,
			"--summary", "bootstrap check")
	}
	out := execute(t, contextLintCmd, nil)
	if strings.Contains(string(out), ctxLintCritical) {
		t.Errorf("lint CRITICAL on bootstrapped files:\n%s", out)
	}
}

func TestContextNewEditNoEditor(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "x", "--edit"))
	})
}

func TestContextList(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	out := execute(t, contextListCmd, nil, "--type", "worklog")
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected empty list, got %q", out)
	}

	execute(t, contextNewCmd, nil, "--type", "worklog", "--slug", "a")
	execute(t, contextNewCmd, nil, "--type", "worklog", "--slug", "b")
	out = execute(t, contextListCmd, nil, "--type", "worklog")
	lines := strings.Fields(string(out))
	if len(lines) != 2 {
		t.Fatalf("expected 2 files, got %q", out)
	}
	wantA := filepath.Join("context", "worklog", "20260806-070000-a.md")
	if lines[0] != wantA {
		t.Errorf("expected %q, got %q", wantA, lines[0])
	}
}

func TestContextListJSON(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "a")
	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "b")

	out := execute(t, contextListCmd, nil, "--type", "plan", "--format", "json")
	var files []string
	if err := json.Unmarshal(out, &files); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if filepath.Base(files[0]) != "20260806-070000-a.md" {
		t.Errorf("unexpected first file: %s", files[0])
	}
}

func TestSanitizeSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Review Deps!":   "review-deps",
		"  ship-Memory ": "ship-memory",
		"a--b__c":        "a--b-c",
		"---":            "",
		"2026-08-06":     "2026-08-06",
	} {
		if got := sanitizeSlug(in); got != want {
			t.Errorf("sanitizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTaskItems(t *testing.T) {
	content := "---\nkind: tasks\n---\n\n- [ ] step one\n- [x] step two\n- [~] step three\n- [!] step four\n- [ ] step five\n"
	items := parseTaskItems(content)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	for i, want := range []string{taskStatusTodo, taskStatusDone, taskStatusWip, taskStatusBlocked, taskStatusTodo} {
		if items[i].Status != want {
			t.Errorf("item %d status = %q, want %q", i, items[i].Status, want)
		}
		if items[i].Line != i+1 {
			t.Errorf("item %d line = %d, want %d", i, items[i].Line, i+1)
		}
	}
	if items[3].Text != "step four" {
		t.Errorf("unexpected text: %q", items[3].Text)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	content := "- [ ] one\n- [ ] two\n"
	got, err := updateTaskStatus(content, 2, taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- [x] two") {
		t.Errorf("expected done marker:\n%s", got)
	}

	got, err = updateTaskStatus(content, 1, taskStatusBlock, "no dep")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- [!] one (blocked: no dep)") {
		t.Errorf("expected blocked marker with reason:\n%s", got)
	}

	if _, err := updateTaskStatus(content, 5, taskStatusDone, ""); err == nil {
		t.Error("expected error for out of range id")
	}
}

func TestFrontmatterField(t *testing.T) {
	if got := frontmatterField("---\nobjective: ship feature\n---\n", "objective"); got != "ship feature" {
		t.Errorf("unexpected objective: %q", got)
	}
	if got := frontmatterField("no frontmatter\n", "objective"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestTaskArchiveSlug(t *testing.T) {
	if got := taskArchiveSlug("Ship Feature Now", ""); got != "ship-feature-now" {
		t.Errorf("expected slug from flag, got %q", got)
	}
	if got := taskArchiveSlug("", "20260912-000000-pipeline.md"); got != "pipeline" {
		t.Errorf("expected slug from plan, got %q", got)
	}
	if got := taskArchiveSlug("", "standalone"); got != "standalone" {
		t.Errorf("expected standalone slug, got %q", got)
	}
	if got := taskArchiveSlug("", ""); got != "tasks" {
		t.Errorf("expected fallback slug, got %q", got)
	}
}

func TestContextTaskLifecycle(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	tasks := func(name string) string {
		return filepath.Join(dir, "context", "tasks", "20260806-070000-custom-phase-"+name+".md")
	}

	// Missing --phase → error.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskListCmd, nil))
	})

	// No active plan and no --plan → error.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskListCmd, nil, "--phase", "1"))
	})

	out := execute(t, contextTaskAddCmd, nil, "build feature", "--phase", "1", "--plan", "custom")
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected id 1, got %q", out)
	}

	execute(t, contextTaskAddCmd, nil, "test feature", "--phase", "1", "--plan", "custom")
	execute(t, contextTaskAddCmd, nil, "release feature", "--phase", "1", "--plan", "custom")

	out = execute(t, contextTaskListCmd, nil, "--phase", "1", "--plan", "custom", "--format", "json")
	var items []taskItem
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	execute(t, contextTaskDoneCmd, nil, "1", "--phase", "1", "--plan", "custom")
	execute(t, contextTaskBlockCmd, nil, "3", "--phase", "1", "--plan", "custom", "--reason", "ci broken")
	execute(t, contextTaskWipCmd, nil, "2", "--phase", "1", "--plan", "custom")

	items = parseTaskItems(mustReadFile(t, tasks("1")))
	want := []string{taskStatusDone, taskStatusWip, taskStatusBlocked}
	for i, w := range want {
		if items[i].Status != w {
			t.Errorf("item %d status = %q, want %q", i, items[i].Status, w)
		}
	}
	if !strings.Contains(mustReadFile(t, tasks("1")), "blocked: ci broken") {
		t.Error("expected block reason in task file")
	}

	out = execute(t, contextTaskArchiveCmd, nil, "--phase", "1", "--plan", "custom")
	archivePath := strings.TrimSpace(string(out))
	if !strings.Contains(archivePath, filepath.Join("context", "archive")) {
		t.Errorf("expected archive path, got %q", out)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("expected archived file at %s: %v", archivePath, err)
	}
	if _, err := os.Stat(tasks("1")); !os.IsNotExist(err) {
		t.Error("expected active task list removed after archive")
	}
}

func TestTaskFileFor(t *testing.T) {
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	cases := []struct{ phase, plan, want string }{
		{"1", "20260911-062956-plan-llm-wiki-pipeline.md", "20260806-070000-plan-llm-wiki-pipeline-phase-1.md"},
		{"1A", "20260911-062956-plan-llm-wiki-pipeline.md", "20260806-070000-plan-llm-wiki-pipeline-phase-1a.md"},
		{"2", "20260912-000000-pipeline.md", "20260806-070000-pipeline-phase-2.md"},
		{"1", "custom", "20260806-070000-custom-phase-1.md"},
	}
	for _, c := range cases {
		got := taskFileFor(c.phase, c.plan)
		want := filepath.Join("context", "tasks", c.want)
		if got != want {
			t.Errorf("taskFileFor(%q, %q) = %q, want %q", c.phase, c.plan, got, want)
		}
	}
}

func TestContextTaskFileStatusTransitions(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	file := func() string {
		return filepath.Join(dir, "context", "tasks", "20260806-070000-custom-phase-1.md")
	}
	status := func() string {
		return frontmatterField(mustReadFile(t, file()), "status")
	}

	execute(t, contextTaskAddCmd, nil, "one", "--phase", "1", "--plan", "custom")
	execute(t, contextTaskAddCmd, nil, "two", "--phase", "1", "--plan", "custom")
	execute(t, contextTaskAddCmd, nil, "three", "--phase", "1", "--plan", "custom")
	if got := status(); got != taskFileStatusPending {
		t.Fatalf("create status = %q, want %q", got, taskFileStatusPending)
	}

	execute(t, contextTaskWipCmd, nil, "1", "--phase", "1", "--plan", "custom")
	if got := status(); got != taskFileStatusInProgress {
		t.Fatalf("wip status = %q, want %q", got, taskFileStatusInProgress)
	}
	execute(t, contextTaskDoneCmd, nil, "1", "--phase", "1", "--plan", "custom")
	if got := status(); got != taskFileStatusInProgress {
		t.Fatalf("done-while-open status = %q, want %q", got, taskFileStatusInProgress)
	}
	execute(t, contextTaskDoneCmd, nil, "2", "--phase", "1", "--plan", "custom")
	execute(t, contextTaskDoneCmd, nil, "3", "--phase", "1", "--plan", "custom")
	if got := status(); got != taskFileStatusCompleted {
		t.Fatalf("done-all status = %q, want %q", got, taskFileStatusCompleted)
	}

	archiveOut := execute(t, contextTaskArchiveCmd, nil, "--phase", "1", "--plan", "custom")
	archived := mustReadFile(t, strings.TrimSpace(string(archiveOut)))
	if got := frontmatterField(archived, "status"); got != taskFileStatusArchived {
		t.Fatalf("archive status = %q, want %q", got, taskFileStatusArchived)
	}
}

func TestTaskSlugFromPlan(t *testing.T) {
	for in, want := range map[string]string{
		"20260911-062956-plan-llm-wiki-pipeline.md": "plan-llm-wiki-pipeline",
		"20260912-000000-pipeline.md":               "pipeline",
		"custom":                                    "custom",
		"custom.md":                                 "custom",
	} {
		if got := taskSlugFromPlan(in); got != want {
			t.Errorf("taskSlugFromPlan(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTaskFileForResolution(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC))
	existing := filepath.Join("context", "tasks", "20290101-000000-x-phase-1.md")
	writeCtxDoc(t, existing, "---\nkind: tasks\nsummary: t\n---\n")
	// Existing file wins over a now-based name (keeps creation-time prefix).
	if got := taskFileFor("1", "x"); got != existing {
		t.Errorf("expected existing phase file, got %q", got)
	}
	// Alphanumeric phase and real plan slug both resolve.
	other := filepath.Join("context", "tasks", "20260806-070000-plan-llm-wiki-pipeline-phase-2b.md")
	writeCtxDoc(t, other, "---\nkind: tasks\nsummary: t\n---\n")
	if got := taskFileFor("2B", "20260911-062956-plan-llm-wiki-pipeline.md"); got != other {
		t.Errorf("expected resolved alphanumeric phase file, got %q", got)
	}
}

func TestContextPathTasks(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	out := execute(t, contextPathCmd, nil, "--type", "tasks", "--phase", "2",
		"--plan", "20260911-062956-plan-llm-wiki-pipeline.md")
	got := strings.TrimSpace(string(out))
	want := filepath.Join("context", "tasks", "20260806-070000-plan-llm-wiki-pipeline-phase-2.md")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestContextPathTasksRequiresPhase(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextPathCmd, nil, "--type", "tasks", "--plan", "custom"))
	})
}

func TestContextTaskAddAutoPlan(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, filepath.Join("context", "plan", "20260912-000000-pipeline.md"),
		"---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")

	execute(t, contextTaskAddCmd, nil, "cd", "--phase", "1")
	file := filepath.Join(dir, "context", "tasks", "20260806-070000-pipeline-phase-1.md")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected auto-plan task file: %v", err)
	}
	content := mustReadFile(t, file)
	if !strings.Contains(content, "links:\n  - plan/20260912-000000-pipeline.md") {
		t.Errorf("expected auto plan link:\n%s", content)
	}
}

func TestContextTaskInvalidID(t *testing.T) {
	runInTempDir(t)
	execute(t, contextTaskAddCmd, nil, "step", "--phase", "1", "--plan", "custom")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskDoneCmd, nil, "abc", "--phase", "1", "--plan", "custom"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskDoneCmd, nil, "9", "--phase", "1", "--plan", "custom"))
	})
}

func TestContextTaskAddEmpty(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskAddCmd, nil, "   "))
	})
}

func TestContextTaskAddFrontmatterConvention(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	// An active plan exists → generated checklist must link it (links+sources).
	writeCtxDoc(t, filepath.Join("context", "plan", "20260912-000000-pipeline.md"),
		"---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")
	execute(t, contextTaskAddCmd, nil, "step one", "--phase", "demo",
		"--summary", "Demo checklist")

	content := mustReadFile(t, filepath.Join(dir, "context", "tasks", "20260806-070000-pipeline-phase-demo.md"))
	for _, want := range []string{
		"kind: tasks",
		"summary: Demo checklist",
		"phase: demo",
		"status: pending",
		"created: 2026-08-06T07:00:00Z",
		"updated: 2026-08-06T07:00:00Z",
		"links:\n  - plan/20260912-000000-pipeline.md",
		"sources:\n  - plan/20260912-000000-pipeline.md",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in generated task frontmatter:\n%s", want, content)
		}
	}
	for _, forbid := range []string{"objective:", "created_at:"} {
		if strings.Contains(content, forbid) {
			t.Errorf("did not expect %q in generated task frontmatter:\n%s", forbid, content)
		}
	}

	// Default summary derives from the phase when --summary is omitted.
	execute(t, contextTaskAddCmd, nil, "step two", "--phase", "other")
	other := mustReadFile(t, filepath.Join(dir, "context", "tasks", "20260806-070000-pipeline-phase-other.md"))
	if !strings.Contains(other, "summary: Task checklist for phase other") {
		t.Errorf("expected derived default summary, got:\n%s", other)
	}

	// lint must not flag the CLI-generated checklist.
	out := execute(t, contextLintCmd, nil)
	if strings.Contains(string(out), "pipeline-phase-demo.md") ||
		strings.Contains(string(out), "pipeline-phase-other.md") {
		t.Errorf("lint flagged CLI-generated task file:\n%s", out)
	}
}

func TestContextTaskAddWritesPhaseNotObjective(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextTaskAddCmd, nil, "step", "--phase", "3a", "--plan", "custom")
	content := mustReadFile(t, filepath.Join("context", "tasks", "20260806-070000-custom-phase-3a.md"))
	if !strings.Contains(content, "phase: 3a") {
		t.Errorf("expected phase: 3a:\n%s", content)
	}
	if strings.Contains(content, "objective:") {
		t.Errorf("task file must not carry objective (inherited from the plan):\n%s", content)
	}
}

func TestContextTaskArchiveEmptyList(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskArchiveCmd, nil))
	})
}

func TestAgentBlockReferencesContextCommands(t *testing.T) {
	block := agentBlockInstructions("", "")
	for _, want := range []string{
		"sdt context reindex",
		"sdt context lint",
		"sdt context task",
		"context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md",
		"context/index.md",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("expected %q in agent block", want)
		}
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
