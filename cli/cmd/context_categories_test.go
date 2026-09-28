package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadCategoryRegisterCanonicalizesAliases(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/categories.yaml", "categories:\n  bug:\n    - bugfix\n  new-feature:\n    - feat\n")

	reg, err := loadCategoryRegister()
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"bugfix", "Bugfix"} {
		if c, ok := reg.canonical(alias); !ok || c != "bug" {
			t.Errorf("alias %q -> (%q,%v), want bug", alias, c, ok)
		}
	}
	if c, ok := reg.canonical("new-feature"); !ok || c != "new-feature" {
		t.Errorf("canonical lookup failed: (%q,%v)", c, ok)
	}
	if _, ok := reg.canonical("nonexistent"); ok {
		t.Error("unknown category must not resolve")
	}
	if got := reg.categorySlugs(); len(got) != 2 || got[0] != "bug" || got[1] != "new-feature" {
		t.Errorf("categorySlugs = %v", got)
	}
}

func TestLoadCategoryRegisterMissingFileIsEmpty(t *testing.T) {
	runInTempDir(t)
	reg, err := loadCategoryRegister()
	if err != nil {
		t.Fatalf("missing register must not error: %v", err)
	}
	if _, ok := reg.canonical("anything"); !ok {
		t.Error("empty register should accept every category")
	}
}

func TestContextNewCategoryFrontmatter(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/categories.yaml", "categories:\n  bug:\n    - bugfix\n  new-feature:\n    - feat\n")

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "c",
		"--category", "bugfix", "--category", "new-feature", "--category", "bugfix")
	content := mustReadFile(t, filepath.Join(dir, "context", "analysis", "20260806-070000-c.md"))
	if !strings.Contains(content, "categories:\n  - bug\n  - new-feature\n") {
		t.Errorf("expected canonicalised, deduped categories:\n%s", content)
	}
}

func TestContextListCategoryFilter(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nlinks: none\ncategories:\n  - bug\n  - refactor\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/b.md", "---\nkind: analysis\nsummary: s\nlinks: none\ncategories:\n  - research\n---\nbody\n")

	out := string(execute(t, contextListCmd, nil, "--type", "analysis", "--category", "bug"))
	if !strings.Contains(out, "a.md") || strings.Contains(out, "b.md") {
		t.Errorf("single-category filter wrong:\n%s", out)
	}

	// repeatable --category is any-match: refactor (a) OR research (b) keeps both
	out = string(execute(t, contextListCmd, nil, "--type", "analysis", "--category", "refactor", "--category", "research"))
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "b.md") {
		t.Errorf("any-match category filter wrong:\n%s", out)
	}

	// alias canonicalisation is a lint concern, not a list filter: an alias is
	// not stored in the corpus, so it matches nothing.
	out = string(execute(t, contextListCmd, nil, "--type", "analysis", "--category", "bugfix"))
	if strings.Contains(out, "a.md") || strings.Contains(out, "b.md") {
		t.Errorf("unregistered alias should match nothing:\n%s", out)
	}
}

func TestContextNewCategoryNotAnalysis(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "plan", "--slug", "x", "--category", "bug"))
	})
}

func TestContextNewCategoryBadSlug(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "x", "--category", "Bad Category"))
	})
}

func TestLintCategoryFields(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/categories.yaml", "categories:\n  bug:\n    - bugfix\n")
	reg, err := loadCategoryRegister()
	if err != nil {
		t.Fatal(err)
	}

	ok := "---\nkind: analysis\nsummary: s\ncategories:\n  - bug\n  - bugfix\n---\nbody\n"
	if got := lintCategoryFields("context/analysis/x.md", ok, reg); len(got) != 0 {
		t.Errorf("expected no issues for canonical+alias categories, got %v", got)
	}

	unknown := "---\nkind: analysis\nsummary: s\ncategories:\n  - made-up\n---\nbody\n"
	got := lintCategoryFields("context/analysis/x.md", unknown, reg)
	if len(got) != 1 || got[0].Priority != ctxLintSuggestion || !strings.Contains(got[0].Message, "unknown category") {
		t.Errorf("expected an unknown-category SUGGESTION, got %v", got)
	}

	bad := "---\nkind: analysis\nsummary: s\ncategories:\n  - Bad Category\n---\nbody\n"
	got = lintCategoryFields("context/analysis/x.md", bad, reg)
	if len(got) != 1 || got[0].Priority != ctxLintWarning {
		t.Fatalf("expected one WARNING, got %v", got)
	}
}

func TestAgentInitSeedsCategoriesRegister(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	if _, err := os.Stat(filepath.Join(dir, ctxCategoriesFilePath)); err == nil {
		t.Fatal("register must not exist before init")
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	body := mustReadFile(t, filepath.Join(dir, ctxCategoriesFilePath))
	if !strings.Contains(body, "context/categories.yaml") || !strings.Contains(body, "categories:") {
		t.Errorf("expected the categories register template, got:\n%s", body)
	}
}
