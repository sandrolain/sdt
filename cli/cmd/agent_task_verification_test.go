//go:build unix

package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/delegation"
)

func TestSelectedVerificationChecksAreOptInAndNameBound(t *testing.T) {
	cfg := &ProjectConfig{Gate: []GateStep{{Name: "unit", Run: "go test ./..."}, {Name: "lint", Run: "task lint"}}}
	checks, err := selectedVerificationChecks(cfg)
	if err != nil || len(checks) != 0 {
		t.Fatalf("absent selection should remain opt-in: checks=%v err=%v", checks, err)
	}
	cfg.Delegation.VerifyOnResult = []string{"lint"}
	checks, err = selectedVerificationChecks(cfg)
	if err != nil || len(checks) != 1 || checks[0].Name != "lint" || checks[0].Run != "task lint" {
		t.Fatalf("selected gate command was not snapshotted: checks=%+v err=%v", checks, err)
	}
	cfg.Delegation.VerifyOnResult = []string{"missing"}
	if _, err := selectedVerificationChecks(cfg); err == nil || !strings.Contains(err.Error(), "unknown or duplicate") {
		t.Fatalf("unknown check was accepted: %v", err)
	}
	cfg.Delegation.VerifyOnResult = []string{"unit", "unit"}
	if _, err := selectedVerificationChecks(cfg); err == nil || !strings.Contains(err.Error(), "unknown or duplicate") {
		t.Fatalf("duplicate check was accepted: %v", err)
	}
	cfg.Gate[0].Exclude = "platform specific"
	cfg.Delegation.VerifyOnResult = []string{"unit"}
	if _, err := selectedVerificationChecks(cfg); err == nil || !strings.Contains(err.Error(), "runnable command") {
		t.Fatalf("excluded check was accepted: %v", err)
	}
}

func TestVerificationFailureAddsPrincipalInboxAction(t *testing.T) {
	store := delegation.NewStore(t.TempDir())
	if err := store.Create(delegation.Task{ID: "verify-inbox", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("verify-inbox", "started", "Worker started", func(task *delegation.Task) error {
		task.Status = delegation.TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("verify-inbox", "result_ready", "Result ready", func(task *delegation.Task) error {
		task.Status = delegation.TaskResultReady
		task.Verification = &delegation.VerificationRun{Status: "failed", CandidateDigest: "digest"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	items, err := store.Inbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !containsVerificationKind(items[0].Kinds, "verification") || !strings.Contains(items[0].NextAction, "Review failed") {
		t.Fatalf("verification failure was not actionable: %+v", items)
	}
}

func containsVerificationKind(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
