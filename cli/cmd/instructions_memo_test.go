package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentInitSeedsMemoRegister(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 10, 4, 7, 0, 0, 0, time.UTC))
	if _, err := os.Stat(filepath.Join(dir, ctxMemoFilePath)); err == nil {
		t.Fatal("register must not exist before init")
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	body := mustReadFile(t, filepath.Join(dir, ctxMemoFilePath))
	if !strings.Contains(body, "context/memo.yaml") || !strings.Contains(body, "rules:") {
		t.Errorf("expected the memo register template, got:\n%s", body)
	}
}

func TestValidateMemoRegisterReportsInvalid(t *testing.T) {
	runInTempDir(t)
	if err := validateMemoRegister(); err != nil {
		t.Fatalf("a missing register must not be a finding: %v", err)
	}

	writeTestFile(t, ctxMemoFilePath, "rules:\n  - id: x\n    summary: s\n    every_days: 0\n")
	if err := validateMemoRegister(); err == nil {
		t.Fatal("want an error for a rule with a non-positive cadence")
	}

	writeTestFile(t, ctxMemoFilePath, "memos:\n  - id: rotate\n    summary: s\n    due: 2026-10-15\n")
	if err := validateMemoRegister(); err != nil {
		t.Fatalf("a valid register must not be a finding: %v", err)
	}
}
