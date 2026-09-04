package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	want := filepath.Join("sdt.context", "worklog", "20260806-070000-review-deps.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "plan")
	want = filepath.Join("sdt.context", "plan", "2026-08-06.md")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	out = execute(t, contextPathCmd, nil, "--type", "analysis", "--slug", "backend-choice")
	want = filepath.Join("sdt.context", "analysis", "2026-08-06-backend-choice.md")
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
	if !strings.HasSuffix(res.Path, "plan/2026-08-06.md") {
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
	want := filepath.Join("sdt.context", "worklog", "20260806-070000-review-deps.md")
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
		"created_at: 2026-08-06T07:00:00Z",
		"context: reviewed deps",
		"reviewed all deps",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in file:\n%s", want, content)
		}
	}
}

func TestContextNewPlan(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "ship-memory", "--input", "body")
	data, err := os.ReadFile(filepath.Join(dir, "sdt.context", "plan", "2026-08-06-ship-memory.md"))
	if err != nil {
		t.Fatalf("expected plan created: %v", err)
	}
	if !strings.Contains(string(data), "kind: plan") {
		t.Errorf("expected kind: plan:\n%s", data)
	}
}

func TestContextNewAnalysis(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "backend-choice", "--input", "body")
	data, err := os.ReadFile(filepath.Join(dir, "sdt.context", "analysis", "2026-08-06-backend-choice.md"))
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
	data, _ := os.ReadFile(filepath.Join(dir, "sdt.context", "notes", "20260806-070000-x.md"))
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
	if res.Type != "worklog" || res.Status != statusCreated || res.CreatedAt == "" {
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
	wantA := filepath.Join("sdt.context", "worklog", "20260806-070000-a.md")
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
	if filepath.Base(files[0]) != "2026-08-06-a.md" {
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
	if got := taskArchiveSlug("---\nobjective: Ship Feature Now\n---\n", ""); got != "ship-feature-now" {
		t.Errorf("expected slug from objective, got %q", got)
	}
	if got := taskArchiveSlug("---\nkind: tasks\n---\n", ""); got != "tasks" {
		t.Errorf("expected fallback slug, got %q", got)
	}
	if got := taskArchiveSlug("anything", "--flag"); got != "flag" {
		t.Errorf("expected flag slug, got %q", got)
	}
}

func TestContextTaskLifecycle(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskListCmd, nil))
	})

	out := execute(t, contextTaskAddCmd, nil, "build feature", "--objective", "ship feature")
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected id 1, got %q", out)
	}

	execute(t, contextTaskAddCmd, nil, "test feature")
	execute(t, contextTaskAddCmd, nil, "release feature")

	out = execute(t, contextTaskListCmd, nil, "--format", "json")
	var items []taskItem
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	execute(t, contextTaskDoneCmd, nil, "1")
	execute(t, contextTaskBlockCmd, nil, "3", "--reason", "ci broken")
	execute(t, contextTaskWipCmd, nil, "2")

	items = parseTaskItems(mustReadFile(t, filepath.Join(dir, "sdt.context", "tasks", "TODO.md")))
	want := []string{taskStatusDone, taskStatusWip, taskStatusBlocked}
	for i, w := range want {
		if items[i].Status != w {
			t.Errorf("item %d status = %q, want %q", i, items[i].Status, w)
		}
	}
	if !strings.Contains(mustReadFile(t, filepath.Join(dir, "sdt.context", "tasks", "TODO.md")), "blocked: ci broken") {
		t.Error("expected block reason in TODO.md")
	}

	out = execute(t, contextTaskArchiveCmd, nil)
	archivePath := strings.TrimSpace(string(out))
	if !strings.Contains(archivePath, filepath.Join("sdt.context", "archive")) {
		t.Errorf("expected archive path, got %q", out)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("expected archived file at %s: %v", archivePath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sdt.context", "tasks", "TODO.md")); !os.IsNotExist(err) {
		t.Error("expected active task list removed after archive")
	}
}

func TestContextTaskInvalidID(t *testing.T) {
	runInTempDir(t)
	execute(t, contextTaskAddCmd, nil, "step")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskDoneCmd, nil, "abc"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskDoneCmd, nil, "9"))
	})
}

func TestContextTaskAddEmpty(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTaskAddCmd, nil, "   "))
	})
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
		"sdt context new --type plan --slug <slug>",
		"sdt context path --type <type>",
		"sdt context task",
		"sdt context task add",
		"sdt context task done|block|wip <id>",
		"sdt context task archive",
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
