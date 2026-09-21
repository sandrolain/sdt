package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadTopicRegisterCanonicalizesAliases(t *testing.T) {
	dir := runInTempDir(t)
	writeCtxDoc(t, "context/topics.yaml", "topics:\n  context-search:\n    - search\n    - retrieval\n  viewer:\n    - web\n")

	reg, err := loadTopicRegister()
	if err != nil {
		t.Fatal(err)
	}
	_ = dir
	for _, alias := range []string{"search", "retrieval"} {
		if c, ok := reg.canonical(alias); !ok || c != "context-search" {
			t.Errorf("alias %q -> (%q,%v), want context-search", alias, c, ok)
		}
	}
	if c, ok := reg.canonical("Context-Search"); !ok || c != "context-search" {
		t.Errorf("canonical lookup failed: (%q,%v)", c, ok)
	}
	if _, ok := reg.canonical("nonexistent"); ok {
		t.Error("unknown topic must not resolve")
	}
	if got := reg.topicSlugs(); len(got) != 2 || got[0] != "context-search" || got[1] != "viewer" {
		t.Errorf("topicSlugs = %v", got)
	}
}

func TestLoadTopicRegisterMissingFileIsEmpty(t *testing.T) {
	runInTempDir(t)
	reg, err := loadTopicRegister()
	if err != nil {
		t.Fatalf("missing register must not error: %v", err)
	}
	if _, ok := reg.canonical("anything"); !ok {
		t.Error("empty register should accept every topic")
	}
}

func TestContextNewTopicEntityFrontmatter(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "analysis", "--slug", "t",
		"--topic", "Context-Search", "--topic", "viewer", "--entity", "bleve")
	content := mustReadFile(t, filepath.Join(dir, "context", "analysis", "20260806-070000-t.md"))
	if !strings.Contains(content, "topics:\n  - context-search\n  - viewer\n") {
		t.Errorf("expected sanitized topics array:\n%s", content)
	}
	if !strings.Contains(content, "entities:\n  - bleve\n") {
		t.Errorf("expected entities array:\n%s", content)
	}
}

func TestLintTopicFields(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/topics.yaml", "topics:\n  context-search:\n    - search\n")
	reg, err := loadTopicRegister()
	if err != nil {
		t.Fatal(err)
	}

	ok := "---\nkind: analysis\nsummary: s\ntopics:\n  - context-search\n  - search\nentities:\n  - bleve\n---\nbody\n"
	if got := lintTopicFields("context/analysis/x.md", ok, reg); len(got) != 0 {
		t.Errorf("expected no issues for canonical+alias topics and valid entity, got %v", got)
	}

	unknown := "---\nkind: analysis\nsummary: s\ntopics:\n  - made-up-topic\n---\nbody\n"
	got := lintTopicFields("context/analysis/x.md", unknown, reg)
	if len(got) != 1 || got[0].Priority != ctxLintSuggestion || !strings.Contains(got[0].Message, "unknown topic") {
		t.Errorf("expected an unknown-topic SUGGESTION, got %v", got)
	}

	bad := "---\nkind: analysis\nsummary: s\ntopics:\n  - Bad Topic\nentities:\n  - Bad Entity\n---\nbody\n"
	got = lintTopicFields("context/analysis/x.md", bad, reg)
	if len(got) != 2 {
		t.Fatalf("expected two WARNINGs, got %v", got)
	}
	for _, it := range got {
		if it.Priority != ctxLintWarning {
			t.Errorf("expected WARNING, got %s (%s)", it.Priority, it.Message)
		}
	}
}

func TestAgentInitSeedsTopicsRegister(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	if _, err := os.Stat(filepath.Join(dir, ctxTopicsFilePath)); err == nil {
		t.Fatal("register must not exist before init")
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	body := mustReadFile(t, filepath.Join(dir, ctxTopicsFilePath))
	if !strings.Contains(body, "context/topics.yaml") || !strings.Contains(body, "topics:") {
		t.Errorf("expected the topics register template, got:\n%s", body)
	}
}

func TestReindexShowsTopicsInIndexLine(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: subject\nobjective: o\nlinks: none\ntopics:\n  - context-search\n---\nbody\n")
	content, err := buildIndex()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "`topics: context-search`") {
		t.Errorf("expected topics in the index line:\n%s", content)
	}
}
