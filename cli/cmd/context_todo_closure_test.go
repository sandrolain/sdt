//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// todoRegister puts one open entry in the register of the working tree.
func todoRegister(t *testing.T) {
	t.Helper()
	writeTestFile(t, filepath.Join("context", "todo.yaml"),
		"items:\n  - id: one\n    text: the first idea\n    created: 2026-10-27\n"+
			"  - id: two\n    text: the second idea\n    created: 2026-10-27\n")
}

// TestNewAnalysisRecordsTheTodosItAnswers pins the first half of the rule: an
// analysis carries the register ids it was opened from, in its frontmatter.
func TestNewAnalysisRecordsTheTodosItAnswers(t *testing.T) {
	runInTempDir(t)
	todoRegister(t)

	path := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", "analysis", "--title", "Lifecycle", "--slug", "lifecycle",
		"--summary", "s", "--todo", "one", "--todo", "two")))
	got := parseFrontmatterList(mustReadFile(t, path), ctxFrontmatterTodos)
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("todos = %v, want [one two]", got)
	}
}

// TestNewRefusesAnUnknownTodoID keeps a typo from becoming a silent no-op: an id the
// register does not know would close nothing at the end of the workflow.
func TestNewRefusesAnUnknownTodoID(t *testing.T) {
	runInTempDir(t)
	todoRegister(t)

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil,
			"--type", "analysis", "--title", "Typo", "--slug", "typo",
			"--summary", "s", "--todo", "nope"))
	})
}

// TestNewPlanInheritsTheTodosAndCarriesTheClosureItem pins the second half: the plan
// created from an analysis inherits the ids and carries the item that closes them, so
// the obligation is visible where the work is planned.
func TestNewPlanInheritsTheTodosAndCarriesTheClosureItem(t *testing.T) {
	runInTempDir(t)
	todoRegister(t)

	analysis := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", "analysis", "--title", "Lifecycle", "--slug", "lifecycle",
		"--summary", "s", "--todo", "one")))
	ref := "analysis/" + strings.TrimSuffix(filepath.Base(analysis), ".md")

	plan := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", "plan", "--title", "Deliver", "--slug", "deliver",
		"--summary", "s", "--source", ref)))
	content := mustReadFile(t, plan)

	if got := parseFrontmatterList(content, ctxFrontmatterTodos); len(got) != 1 || got[0] != "one" {
		t.Errorf("a plan inherits the analysis todos: %v", got)
	}
	if !strings.Contains(content, "## Todo closure") || !strings.Contains(content, "`one`") {
		t.Errorf("the plan must carry the closing item:\n%s", content)
	}
}

// TestCloseClosesTheTodosTheAnalysisCarries pins the end: closing the document that
// carries the ids marks them done in the register, once.
func TestCloseClosesTheTodosTheAnalysisCarries(t *testing.T) {
	runInTempDir(t)
	todoRegister(t)

	analysis := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", "analysis", "--title", "Lifecycle", "--slug", "lifecycle",
		"--summary", "s", "--todo", "one")))

	out := string(execute(t, contextCloseCmd, nil, analysis))
	if !strings.Contains(out, "todos closed: one") {
		t.Fatalf("the close names what it closed: %q", out)
	}
	reg := todoRegisterOnDisk(t)
	if !reg["one"] {
		t.Error("the todo the analysis answered is still open")
	}
	if reg["two"] {
		t.Error("a todo the document never named must stay open")
	}

	// Idempotent: a second close changes nothing and says nothing.
	out = string(execute(t, contextCloseCmd, nil, analysis))
	if strings.Contains(out, "todos closed") {
		t.Errorf("a second close must not claim a change: %q", out)
	}
}

// todoRegisterOnDisk reads the done state of the register.
func todoRegisterOnDisk(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("context", "todo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	var current string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "- id: "):
			current = strings.TrimPrefix(line, "- id: ")
			out[current] = false
		case line == "done: true" && current != "":
			out[current] = true
		}
	}
	return out
}
