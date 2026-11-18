//go:build unix

package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/delegation"
)

func TestAgentTaskCommandContractAndBoundedStatus(t *testing.T) {
	for _, path := range [][]string{
		{"agent", "task", "start"}, {"agent", "task", "status"}, {"agent", "task", "result"},
		{"agent", "task", "cancel"}, {"agent", "task", "resume"},
		{"agent", "task", "accept"}, {"agent", "task", "correct"},
		{"agent", "task", "apply", "preview"}, {"agent", "task", "apply", "execute"},
		{"agent", "task", "cleanup", "preview"}, {"agent", "task", "cleanup", "execute"},
		{"agent", "task", "permissions", "list"}, {"agent", "task", "permissions", "resolve"},
		{"agent", "task", "availability", "list"}, {"agent", "task", "availability", "reset"},
	} {
		if _, _, err := rootCmd.Find(path); err != nil {
			t.Fatalf("command %v not registered: %v", path, err)
		}
	}
	now := time.Now().UTC()
	task := delegation.Task{ID: "bounded", Status: delegation.TaskResultReady, Role: "reviewer", Deadline: now.Add(time.Hour),
		Attempts:    []delegation.Attempt{{ID: "1", StartedAt: now.Add(-time.Second)}, {ID: "2", StartedAt: now.Add(-2 * time.Second)}, {ID: "3", StartedAt: now.Add(-3 * time.Second)}, {ID: "4", StartedAt: now.Add(-4 * time.Second)}},
		Events:      []delegation.StateEvent{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}},
		Permissions: []delegation.PermissionRecord{{RequestID: "pending", Disposition: "pending"}},
	}
	view := boundedTaskStatus(task)
	if view.AttemptCount != 4 || len(view.Attempts) != 3 || len(view.RecentEvents) != 5 || len(view.PendingPermissions) != 1 {
		t.Fatalf("status summary was not bounded or lost actionable state: %+v", view)
	}
	if view.Metrics.PrincipalTokens != "unknown" || view.Metrics.DelegateTokens != "unknown" || view.Metrics.PrincipalCostUSD != "unknown" || view.Metrics.DelegateCostUSD != "unknown" || view.Metrics.TotalCostUSD != "unknown" || view.Metrics.CommunicationBytes != "unknown" {
		t.Fatalf("unobserved metrics were not labeled unknown: %+v", view.Metrics)
	}
}

func TestMaterializeDelegationReferencesCopiesStableContentAndRejectsEscape(t *testing.T) {
	root, workspace := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("stable source"), 0o600); err != nil {
		t.Fatal(err)
	}
	references, err := materializeDelegationReferences(root, workspace, []string{"docs/guide.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 || references[0].WorkspacePath != "context/delegation-references/docs/guide.md" || references[0].SHA256 == "" {
		t.Fatalf("reference provenance missing: %+v", references)
	}
	copy, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(references[0].WorkspacePath)))
	if err != nil || string(copy) != "stable source" {
		t.Fatalf("stable reference copy = %q, %v", copy, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeDelegationReferences(root, workspace, []string{"escape.md"}); err == nil {
		t.Fatal("symlink reference escaping the project was accepted")
	}
}
