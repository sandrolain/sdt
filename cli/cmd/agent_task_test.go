//go:build unix

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/delegation"
)

func TestAgentTaskCommandContractAndBoundedStatus(t *testing.T) {
	for _, path := range [][]string{
		{"agent", "task", "start"}, {"agent", "task", "status"}, {"agent", "task", "result"}, {"agent", "task", "wait"},
		{"agent", "task", "cancel"}, {"agent", "task", "resume"},
		{"agent", "task", "accept"}, {"agent", "task", "correct"}, {"agent", "task", "fail"}, {"agent", "task", "discard"},
		{"agent", "task", "inbox", "list"},
		{"agent", "task", "permissions", "resolve-batch"},
		{"agent", "task", "apply", "preview"}, {"agent", "task", "apply", "execute"},
		{"agent", "task", "cleanup", "preview"}, {"agent", "task", "cleanup", "execute"},
		{"agent", "task", "permissions", "list"}, {"agent", "task", "permissions", "resolve"},
		{"agent", "task", "availability", "list"}, {"agent", "task", "availability", "reset"},
	} {
		if _, _, err := rootCmd.Find(path); err != nil {
			t.Fatalf("command %v not registered: %v", path, err)
		}
	}
	t.Setenv(delegationWorkerEnv, "nested-parent")
	if err := runAgentTaskStart(nil, nil); err == nil || !strings.Contains(err.Error(), "nested SDT task dispatch is disabled inside delegated task nested-parent") {
		t.Fatalf("nested task dispatch guard returned %v", err)
	}
	now := time.Now().UTC()
	task := delegation.Task{ID: "bounded", Status: delegation.TaskResultReady, Role: "reviewer", Deadline: now.Add(time.Hour),
		Attempts:    []delegation.Attempt{{ID: "1", StartedAt: now.Add(-time.Second)}, {ID: "2", StartedAt: now.Add(-2 * time.Second)}, {ID: "3", StartedAt: now.Add(-3 * time.Second)}, {ID: "4", StartedAt: now.Add(-4 * time.Second)}},
		Events:      []delegation.StateEvent{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}},
		Permissions: []delegation.PermissionRecord{{RequestID: "pending", Disposition: "pending"}},
	}
	task.ConfigSnapshot = []byte(`{"max_attempts":5}`)
	view := boundedTaskStatus(task)
	if view.MaxAttempts != 5 || view.ProviderQuota != "unknown" {
		t.Fatalf("unexpected local limit or provider quota: %+v", view)
	}
	if view.AttemptCount != 4 || len(view.Attempts) != 3 || len(view.RecentEvents) != 5 || len(view.PendingPermissions) != 1 {
		t.Fatalf("status summary was not bounded or lost actionable state: %+v", view)
	}
	if view.NextAction != "review the result, then accept or request a correction" {
		t.Fatalf("result-ready task has wrong next action %q", view.NextAction)
	}
	if view.Metrics.PrincipalTokens != "unknown" || view.Metrics.DelegateTokens != "unknown" || view.Metrics.PrincipalCostUSD != "unknown" || view.Metrics.DelegateCostUSD != "unknown" || view.Metrics.TotalCostUSD != "unknown" || view.Metrics.CommunicationBytes != "unknown" {
		t.Fatalf("unobserved metrics were not labeled unknown: %+v", view.Metrics)
	}
}

func TestPrincipalNextActionAccountsForIntegrationReceipt(t *testing.T) {
	tests := []struct {
		name string
		task delegation.Task
		want string
	}{
		{name: "accepted before apply", task: delegation.Task{Status: delegation.TaskAccepted}, want: "preview and digest-check apply"},
		{name: "accepted after apply", task: delegation.Task{Status: delegation.TaskAccepted, Integration: &delegation.IntegrationReceipt{Digest: "digest", Applied: time.Now()}}, want: "verify the applied changes and update the linked plan"},
		{name: "completed", task: delegation.Task{Status: delegation.TaskCompleted}, want: "verify the applied changes and update the linked plan"},
		{name: "result ready", task: delegation.Task{Status: delegation.TaskResultReady}, want: "review the result, then accept or request a correction"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := principalNextAction(test.task); got != test.want {
				t.Fatalf("principalNextAction() = %q, want %q", got, test.want)
			}
		})
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

func TestValidateTaskPlanReferenceSupportsPlanWideAndPhaseScopes(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "context", "plan", "example.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("## Phase 1 — First\n- [ ] step <!-- c1 -->\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, reference, phase string
		wantReference          string
		wantErr                string
	}{
		{name: "unlinked task"},
		{name: "whole plan", reference: "context/plan/example.md", wantReference: "context/plan/example.md"},
		{name: "phase checklist", reference: "context/plan/example.md", phase: "c1", wantReference: "context/plan/example.md"},
		{name: "phase heading", reference: "context/plan/example.md", phase: "Phase 1", wantReference: "context/plan/example.md"},
		{name: "unknown phase", reference: "context/plan/example.md", phase: "Phase 2", wantErr: "plan phase"},
		{name: "phase without plan", phase: "c1", wantErr: "--phase requires --plan"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateTaskPlanReference(root, tc.reference, tc.phase)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("validateTaskPlanReference() error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTaskPlanReference() error = %v", err)
			}
			if got != tc.wantReference {
				t.Fatalf("validateTaskPlanReference() = %q, want %q", got, tc.wantReference)
			}
		})
	}
}

func TestTaskPlanScopePresentation(t *testing.T) {
	if got := taskPlanScope("context/plan/example.md", ""); got != "entire plan" {
		t.Fatalf("plan-wide task scope = %q, want %q", got, "entire plan")
	}
	if got := taskPlanScope("context/plan/example.md", "c1"); got != "c1" {
		t.Fatalf("phase task scope = %q, want %q", got, "c1")
	}
	if got := formatPlanScopeLine("", ""); got != "" {
		t.Fatalf("unlinked task rendered a plan label: %q", got)
	}
}
