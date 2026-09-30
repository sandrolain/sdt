package ctxvocab

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSlugMatchesTheKebabCaseGrammar(t *testing.T) {
	cases := map[string]string{
		" Viewer ":     "viewer",
		"ui-ux":        "ui-ux",
		"UI UX":        "ui-ux",
		"bug--fix":     "bug--fix",
		"refactor.tmp": "refactor-tmp",
		"":             "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	if !SlugRegexp().MatchString("viewer") || !SlugRegexp().MatchString("ui-ux") {
		t.Error("SlugRegexp rejects a valid slug")
	}
	if SlugRegexp().MatchString("Viewer") || SlugRegexp().MatchString("ui_ux") {
		t.Error("SlugRegexp accepts an invalid slug")
	}
}

func TestLoadCategoriesCanonicalizesAliases(t *testing.T) {
	root := t.TempDir()
	write(t, root, CategoriesFile, "categories:\n  bug: [defect]\n  refactor: []\n  New Feature: []\n")
	reg, err := LoadCategories(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.Slugs(); len(got) != 3 || got[0] != "bug" || got[2] != "refactor" {
		t.Errorf("slugs = %v", got)
	}
	for _, alias := range []string{"bug", "defect"} {
		c, ok := reg.Canonical(alias)
		if !ok || c != "bug" {
			t.Errorf("Canonical(%q) = %q, %v", alias, c, ok)
		}
	}
	if _, ok := reg.Canonical("nope"); ok {
		t.Error("unknown category reported as known")
	}
}

func TestLoadTopicsAndMissingRegisters(t *testing.T) {
	root := t.TempDir()
	write(t, root, TopicsFile, "topics:\n  viewer: []\n")
	topics, err := LoadTopics(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics.Slugs()) != 1 || topics.Slugs()[0] != "viewer" {
		t.Errorf("topics = %v", topics.Slugs())
	}
	// a missing register is empty, never an error
	cats, err := LoadCategories(root)
	if err != nil {
		t.Fatalf("missing register errored: %v", err)
	}
	if len(cats.Slugs()) != 0 {
		t.Errorf("missing register = %v", cats.Slugs())
	}
	var nilReg *Register
	if nilReg.Slugs() != nil {
		t.Error("nil register returned slugs")
	}
	if _, ok := nilReg.Canonical("x"); ok {
		t.Error("nil register reported a known value")
	}
}

func TestLoadRejectsAMalformedRegister(t *testing.T) {
	root := t.TempDir()
	write(t, root, CategoriesFile, "categories: [oops\n")
	if _, err := LoadCategories(root); err == nil {
		t.Error("malformed register accepted")
	}
}

func TestObjectivesListsTheGroupDocumentsSorted(t *testing.T) {
	root := t.TempDir()
	// a group is a document: context/objectives/<id>.md
	write(t, root, ObjectivesDir+"/viewer.md", "objective: viewer\n")
	write(t, root, ObjectivesDir+"/cli.md", "objective: cli\n")
	write(t, root, ObjectivesDir+"/README.md", "not a group\n")
	write(t, root, ObjectivesDir+"/Not A Group.md", "invalid stem\n")
	if err := os.MkdirAll(filepath.Join(root, ObjectivesDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Objectives(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "cli" || got[1] != "viewer" {
		t.Errorf("objectives = %v", got)
	}
	// a project without objectives is empty, not an error
	empty, err := Objectives(t.TempDir())
	if err != nil || len(empty) != 0 {
		t.Errorf("missing objectives = %v, %v", empty, err)
	}
}
