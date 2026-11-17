//go:build unix

package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/sandrolain/sdt/internal/acpbridge"
	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

func runningTask(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("task-1", "attempt_started", "Attempt started", func(task *Task) error {
		task.Status = TaskRunning
		task.Attempts = []Attempt{{ID: "attempt-1", Agent: "reviewer", Model: "test-model", Status: "running"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return store, filepath.Join(root, "workspace")
}

func TestRunACPAttemptCapturesVisibleResultAndDoesNotAcceptIt(t *testing.T) {
	store, workspace := runningTask(t)
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	peer := acptest.NewAgent(t, acptest.Streams("visible answer"))
	err := RunACPAttempt(context.Background(), ACPWorkerOptions{
		Store: store, TaskID: "task-1", AttemptID: "attempt-1", Agent: "reviewer",
		Command: []string{"unused-test-command"}, Workspace: workspace, Brief: "Review this",
		bridgeStart: func(ctx context.Context, bridge *acpbridge.Bridge, _ []string, _ string) (string, error) {
			return bridge.Activate(ctx, peer.Path())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskResultReady || task.PrincipalDecision != nil || len(task.Artifacts) != 1 {
		t.Fatalf("turn completion incorrectly changed acceptance or lost artifact: %+v", task)
	}
	result, err := os.ReadFile(task.Artifacts[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "visible answer") || !strings.Contains(string(result), "Pending principal review") || strings.Contains(string(result), "thought") {
		t.Fatalf("result artifact has wrong content: %s", result)
	}
}

func TestModelRejectionFailsACPAttempt(t *testing.T) {
	store, workspace := runningTask(t)
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	peer := acptest.NewAgent(t, func(agent *acptest.Agent, method string, params json.RawMessage) any {
		if method == "session/set_config_option" {
			return acptest.RPCError{Code: -32602, Message: "model rejected"}
		}
		return acptest.AgentVersion(acp.ProtocolVersionNumber, "sess-1")(agent, method, params)
	})
	err := RunACPAttempt(context.Background(), ACPWorkerOptions{
		Store: store, TaskID: "task-1", AttemptID: "attempt-1", Agent: "reviewer", Model: "rejected-model",
		Command: []string{"unused-test-command"}, Workspace: workspace, Brief: "Review this",
		bridgeStart: func(ctx context.Context, bridge *acpbridge.Bridge, _ []string, _ string) (string, error) {
			return bridge.Activate(ctx, peer.Path())
		},
	})
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("model rejection was not surfaced as an attempt failure: %v", err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskFailed || len(task.Artifacts) != 0 {
		t.Fatalf("rejected model left a result or wrong task state: %+v", task)
	}
}

func TestAutomaticPermissionRequiresMatchingOnceOptionAndOmitsRawInput(t *testing.T) {
	store, _ := runningTask(t)
	opts := ACPWorkerOptions{
		Store: store, TaskID: "task-1", AttemptID: "attempt-1", Agent: "reviewer", Model: "m1",
		Policy: func(context.Context, string) (PermissionRule, error) {
			return PermissionRule{Capability: "edit", Decision: "allow", Version: "v1"}, nil
		},
	}
	request := acpbridge.Permission{
		RequestID: "acp-perm-1", ToolCallID: "call-1", ToolKind: "edit", Title: "write file",
		Input: map[string]any{"secret": "must not persist"},
		Options: []acpbridge.PermissionOption{
			{OptionID: "always", Kind: "allow_always"},
			{OptionID: "once", Kind: "allow_once"},
			{OptionID: "reject", Kind: "reject_once"},
		},
	}
	option, err := opts.handlePermission(context.Background(), request)
	if err != nil || option != "once" {
		t.Fatalf("automatic policy selected %q, %v; want exact allow-once option", option, err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Permissions) != 1 || task.Permissions[0].Disposition != "resolved" || task.Permissions[0].Source != "automatic" || task.Permissions[0].PolicyVersion != "v1" {
		t.Fatalf("automatic permission decision was not recorded: %+v", task.Permissions)
	}
	data, err := os.ReadFile(store.statePath("task-1"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "must not persist") {
		t.Fatal("raw permission input leaked into durable state")
	}
}

func TestAutomaticDenySelectsOnlyRejectOnce(t *testing.T) {
	store, _ := runningTask(t)
	opts := ACPWorkerOptions{
		Store: store, TaskID: "task-1", AttemptID: "attempt-1", Agent: "reviewer",
		Policy: func(context.Context, string) (PermissionRule, error) {
			return PermissionRule{Capability: "terminal", Decision: "deny", Version: "v1"}, nil
		},
	}
	option, err := opts.handlePermission(context.Background(), acpbridge.Permission{
		RequestID: "acp-perm-2", ToolKind: "terminal", Options: []acpbridge.PermissionOption{
			{OptionID: "reject-always", Kind: "reject_always"},
			{OptionID: "reject-once", Kind: "reject_once"},
			{OptionID: "allow-once", Kind: "allow_once"},
		},
	})
	if err != nil || option != "reject-once" {
		t.Fatalf("automatic denial selected %q, %v; want the exact one-request refusal", option, err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || len(task.Permissions) != 1 || task.Permissions[0].OptionID != "reject-once" || task.Permissions[0].Source != "automatic" {
		t.Fatalf("automatic denial changed task authority or was not recorded: %+v", task)
	}
}

func TestPermissionDecisionRequiresExactCurrentAttemptAndOfferedOption(t *testing.T) {
	store, _ := runningTask(t)
	if err := store.RecordPermission("task-1", PermissionRecord{
		RequestID: "req-1", AttemptID: "attempt-1", Capability: "edit", Scope: "edit: file",
		Options: []PermissionChoice{{OptionID: "yes", Kind: "allow_once"}}, Disposition: "pending", PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ResolvePermission("task-1", "old-attempt", "req-1", "yes", "principal", "within mandate", "v1", ""); err == nil {
		t.Fatal("stale attempt resolved a request")
	}
	if err := store.ResolvePermission("task-1", "attempt-1", "req-1", "invented", "principal", "within mandate", "v1", ""); err == nil {
		t.Fatal("unoffered option resolved a request")
	}
	if err := store.ResolvePermission("task-1", "attempt-1", "req-1", "yes", "principal", "within mandate", "v1", ""); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || task.Permissions[0].OptionID != "yes" || task.Permissions[0].Source != "principal" {
		t.Fatalf("decision was not durably resolved: %+v", task)
	}
	if err := store.ResolvePermission("task-1", "attempt-1", "req-1", "yes", "principal", "duplicate", "v1", ""); err == nil {
		t.Fatal("late duplicate resolution changed an already resolved request")
	}
}

func TestPolicyChangeMarksPendingPermissionStaleWithoutApproval(t *testing.T) {
	store, _ := runningTask(t)
	if err := store.RecordPermission("task-1", PermissionRecord{
		RequestID: "req-2", AttemptID: "attempt-1", Capability: "edit", Scope: "edit: file",
		Options: []PermissionChoice{{OptionID: "yes", Kind: "allow_once"}}, Disposition: "pending", PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ResolvePermission("task-1", "attempt-1", "req-2", "yes", "principal", "within mandate", "v2", ""); err == nil {
		t.Fatal("answer under a changed policy was accepted")
	}
	if err := store.MarkPermissionStale("task-1", "attempt-1", "req-2", "v2"); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskInterventionRequired || task.Permissions[0].Disposition != "stale" || task.Permissions[0].OptionID != "" {
		t.Fatalf("policy change granted an approval or lost intervention: %+v", task)
	}
}
