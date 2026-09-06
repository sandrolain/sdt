package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupContextProject(t *testing.T) string {
	t.Helper()
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	return dir
}

func writeCtxDoc(t *testing.T, rel, frontmatter string) {
	t.Helper()
	path := filepath.Join(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContextReindex(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/plan/planx.md", "---\nkind: plan\nsummary: A plan summary\ncreated_at: 2026-01-01T00:00:00Z\n---\nbody\n")
	out := execute(t, contextReindexCmd, nil)
	if !strings.Contains(string(out), "sdt.context/index.md") {
		t.Fatalf("expected index path in output: %s", out)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "sdt.context/index.md"))
	content := string(idx)
	if !strings.Contains(content, "[[plan/planx.md]]") {
		t.Errorf("expected plan doc in index:\n%s", content)
	}
	if !strings.Contains(content, "A plan summary") {
		t.Errorf("expected summary in index:\n%s", content)
	}
}

func TestContextReindexSkipUnchanged(t *testing.T) {
	setupContextProject(t)
	first := execute(t, contextReindexCmd, nil)
	second := execute(t, contextReindexCmd, nil)
	if strings.Contains(string(second), "(written)") && !strings.Contains(string(first), "(written)") {
		t.Errorf("expected second run skipped")
	}
	if !strings.Contains(string(first), "(written)") {
		t.Errorf("expected first run written: %s", first)
	}
}

func TestContextLintClean(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/plan/good.md", "---\nkind: plan\nsummary: Good plan\nlinks:\n  - notes/other.md\n---\nbody\n")
	writeCtxDoc(t, "sdt.context/notes/other.md", "---\nkind: notes\nsummary: Other\n---\nbody\n")
	idx := "---\nkind: index\nsummary: index\n"
	idx += "[[plan/good.md]]\n"
	idx += "[[notes/other.md]]\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), `"CRITICAL"`) {
		t.Errorf("expected no CRITICAL issues: %s", out)
	}
}

func TestContextLintMissingSummary(t *testing.T) {
	setupContextProject(t)
	path := "sdt.context/plan/nosummary.md"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nkind: plan\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextLintCmd, nil, "--format", "json")
		return ""
	})
}

func TestContextLintBrokenLink(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/plan/nosearch.md", "---\nkind: plan\nsummary: x\n[[missing-file]]\n---\nbody\n")
	idx := "---\nkind: index\nsummary: i\n"
	idx += "[[plan/nosearch.md]]\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken link") {
		t.Errorf("expected broken link warning: %s", out)
	}
}

func TestContextStatus(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextStatusCmd, nil)
	for _, typ := range []string{"architecture", "decisions", "plan", "tasks"} {
		if !strings.Contains(string(out), typ+":") {
			t.Errorf("expected type %q in status: %s", typ, out)
		}
	}
}

func TestContextTemplate(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextTemplateCmd, nil, "--type", "adr")
	if !strings.Contains(string(out), "decisions/") && !strings.Contains(string(out), "ADR") {
		t.Errorf("expected ADR template content: %s", out)
	}
}

func TestContextTemplateUnknownType(t *testing.T) {
	setupContextProject(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextTemplateCmd, nil, "--type", "bogus")
		return ""
	})
}

func TestContextListArchitectureAndDecisions(t *testing.T) {
	dir := setupContextProject(t)
	//#nosec G306 -- user work file
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/architecture/stack.md"), []byte("---\nkind: architecture\nsummary: stack\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	//#nosec G306 -- user work file
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/decisions/0001-x.md"), []byte("---\nkind: adr\nnumber: 0001\nsummary: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextListCmd, nil, "--type", "architecture")
	if !strings.Contains(string(out), "stack.md") {
		t.Errorf("expected stack.md in architecture list: %s", out)
	}
	out = execute(t, contextListCmd, nil, "--type", "decisions")
	if !strings.Contains(string(out), "0001-x.md") {
		t.Errorf("expected 0001-x.md in decisions list: %s", out)
	}
}

func TestContextTaskPhaseFile(t *testing.T) {
	dir := setupContextProject(t)
	execute(t, contextTaskAddCmd, nil, "step exec", "--phase", "execution")
	if _, err := os.Stat(filepath.Join(dir, "sdt.context/tasks/execution.md")); err != nil {
		t.Fatalf("expected tasks/execution.md to exist: %v", err)
	}
	out := execute(t, contextTaskListCmd, nil, "--phase", "execution", "--format", "json")
	if !strings.Contains(string(out), "step exec") {
		t.Errorf("expected step in execution list: %s", out)
	}
}

func TestContextTaskArchivePhaseFile(t *testing.T) {
	dir := setupContextProject(t)
	execute(t, contextTaskAddCmd, nil, "one", "--phase", "verify")
	out := execute(t, contextTaskArchiveCmd, nil, "--phase", "verify")
	archivePath := strings.TrimSpace(string(out))
	if !strings.Contains(archivePath, filepath.Join("sdt.context", "archive")) {
		t.Errorf("expected archive path, got %q", out)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archived file at %s: %v", archivePath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sdt.context/tasks/verify.md")); !os.IsNotExist(err) {
		t.Error("expected task file removed after archive")
	}
}

func TestContextQuestionsPath(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextPathCmd, nil, "--type", "questions", "--slug", "backend")
	if !strings.Contains(string(out), filepath.Join("sdt.context", "questions")) {
		t.Errorf("expected questions path, got %s", out)
	}
	if !strings.Contains(string(out), "-backend.md") {
		t.Errorf("expected slug in questions path, got %s", out)
	}
}

func TestContextQuestionsNew(t *testing.T) {
	dir := setupContextProject(t)
	execute(t, contextNewCmd, nil, "--type", "questions", "--slug", "open-api", "--input", "body")
	path := filepath.Join(dir, "sdt.context", "questions")
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("expected questions dir: %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Name(), "open-api") {
		t.Fatalf("expected one questions file with slug, got %v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(path, entries[0].Name()))
	if !strings.Contains(string(data), "kind: questions") {
		t.Errorf("expected kind: questions in doc:\n%s", data)
	}
}

func TestContextQuestionsTemplate(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextTemplateCmd, nil, "--type", "questions")
	if !strings.Contains(string(out), "kind: questions") {
		t.Errorf("expected questions template content: %s", out)
	}
	if !strings.Contains(string(out), "sources") {
		t.Errorf("expected sources field in questions template: %s", out)
	}
}

func TestContextStatusIncludesQuestions(t *testing.T) {
	setupContextProject(t)
	out := execute(t, contextStatusCmd, nil)
	if !strings.Contains(string(out), "questions:") {
		t.Errorf("expected questions type in status: %s", out)
	}
}

func TestContextReindexIncludesQuestions(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/questions/q.md", "---\nkind: questions\nsummary: An open question\n---\nbody\n")
	execute(t, contextReindexCmd, nil)
	idx, _ := os.ReadFile(filepath.Join(dir, "sdt.context/index.md"))
	if !strings.Contains(string(idx), "[[questions/q.md]]") {
		t.Errorf("expected questions doc in index:\n%s", idx)
	}
}

func TestContextLintSourcesResolves(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/analysis/base.md", "---\nkind: analysis\nsummary: base\n---\n")
	writeCtxDoc(t, "sdt.context/questions/q.md", "---\nkind: questions\nsummary: q\nsources:\n  - analysis/base.md\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), "broken source") {
		t.Errorf("expected no broken source warnings for valid reference: %s", out)
	}
}

func TestContextLintSourcesMissingOnDerived(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/questions/q.md", "---\nkind: questions\nsummary: q\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "missing `sources`") {
		t.Errorf("expected missing sources warning on derived questions doc: %s", out)
	}
}

func TestContextLintSourcesBroken(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "sdt.context/questions/q.md", "---\nkind: questions\nsummary: q\nsources:\n  - analysis/does-not-exist.md\n---\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "sdt.context/index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "broken source") {
		t.Errorf("expected broken source warning: %s", out)
	}
}
