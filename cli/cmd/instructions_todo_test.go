package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentInitSeedsTodoRegister(t *testing.T) {
	dir := runInTempDir(t)
	if _, err := os.Stat(filepath.Join(dir, ctxTodoFilePath)); err == nil {
		t.Fatal("register must not exist before init")
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	body := mustReadFile(t, filepath.Join(dir, ctxTodoFilePath))
	if !strings.Contains(body, "context/todo.yaml") || !strings.Contains(body, "items:") {
		t.Errorf("expected the todo register template, got:\n%s", body)
	}
}

func TestValidateTodoRegisterReportsInvalid(t *testing.T) {
	runInTempDir(t)
	if err := validateTodoRegister(); err != nil {
		t.Fatalf("a missing register must not be a finding: %v", err)
	}

	writeTestFile(t, ctxTodoFilePath, "items:\n  - id: a\n    text: x\n")
	if err := validateTodoRegister(); err == nil {
		t.Fatal("want an error for an item with no created date")
	}

	writeTestFile(t, ctxTodoFilePath, "items:\n  - id: a\n    text: x\n    created: 2026-10-04\n")
	if err := validateTodoRegister(); err != nil {
		t.Fatalf("a valid register must not be a finding: %v", err)
	}
}
