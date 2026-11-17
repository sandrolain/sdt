//go:build unix

package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

func newQueuedTask(t *testing.T, required []string) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Create(Task{ID: "chain-task", Role: "reviewer", RequiredCapabilities: required, Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	return store, workspace
}

func TestRunACPTaskFailsOverInConfiguredOrderAndRetainsHandoff(t *testing.T) {
	store, workspace := newQueuedTask(t, []string{"file-read"})
	firstBase := acptest.OffersModels("model-a", "model-a")
	first := acptest.NewAgent(t, func(agent *acptest.Agent, method string, params json.RawMessage) any {
		if method == "session/prompt" {
			return acptest.RPCError{Code: -32603, Message: "provider response", Data: map[string]any{"type": "quota_exceeded"}}
		}
		return firstBase(agent, method, params)
	})
	secondStream := acptest.Streams("replacement result")
	second := acptest.NewAgent(t, func(agent *acptest.Agent, method string, params json.RawMessage) any {
		if method == "session/prompt" {
			return secondStream(agent, method, params)
		}
		return acptest.OffersModels("model-b", "model-b")(agent, method, params)
	})
	candidates := []ACPAgentCandidate{
		{Name: "first", Command: []string{"model-a"}, Model: "model-a", Roles: []string{"reviewer"}, Capabilities: []string{"file-read"}, Concurrency: 1},
		{Name: "second", Command: []string{"model-b"}, Model: "model-b", Roles: []string{"reviewer"}, Capabilities: []string{"file-read", "search"}, Concurrency: 1},
		{Name: "wrong-capability", Model: "model-c", Roles: []string{"reviewer"}},
	}
	var starts []string
	err := RunACPTask(context.Background(), ACPTaskOptions{
		Store: store, TaskID: "chain-task", Workspace: workspace, Brief: "Investigate",
		Candidates: candidates, MaxAttempts: 2, bridgeStart: func(ctx context.Context, bridge *acpbridge.Bridge, command []string, socket string) (string, error) {
			if command[0] == "model-a" {
				starts = append(starts, command[0])
				return bridge.Activate(ctx, first.Path())
			}
			starts = append(starts, command[0])
			return bridge.Activate(ctx, second.Path())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || starts[0] != "model-a" || starts[1] != "model-b" {
		t.Fatalf("candidate order/capability filter = %v", starts)
	}
	task, err := store.Read("chain-task")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Attempts) != 2 || task.Status != TaskResultReady || len(task.Artifacts) != 2 {
		t.Fatalf("failover did not preserve both attempts and final result: %+v", task)
	}
	if task.Artifacts[0].Name != "handoff-attempt-1" {
		t.Fatalf("handoff was not retained before replacement result: %+v", task.Artifacts)
	}
}

func TestRunACPTaskStopsWhenHandoffCannotBePublished(t *testing.T) {
	store, workspace := newQueuedTask(t, nil)
	base := acptest.OffersModels("a", "a")
	first := acptest.NewAgent(t, func(agent *acptest.Agent, method string, params json.RawMessage) any {
		if method == "session/prompt" {
			return acptest.RPCError{Code: -32603, Message: "provider response", Data: map[string]any{"type": "service_unavailable"}}
		}
		return base(agent, method, params)
	})
	second := acptest.NewAgent(t, acptest.OffersModels("b", "b"))
	// Force the atomic publication to fail without corrupting another artifact.
	artifacts := filepath.Join(store.taskDir("chain-task"), "artifacts")
	if err := os.Remove(artifacts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifacts, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondStarted := false
	err := RunACPTask(context.Background(), ACPTaskOptions{
		Store: store, TaskID: "chain-task", Workspace: workspace, Candidates: []ACPAgentCandidate{
			{Name: "first", Command: []string{"a"}, Model: "a", Roles: []string{"reviewer"}, Concurrency: 1},
			{Name: "second", Command: []string{"b"}, Model: "b", Roles: []string{"reviewer"}, Concurrency: 1},
		}, MaxAttempts: 2,
		bridgeStart: func(ctx context.Context, bridge *acpbridge.Bridge, command []string, _ string) (string, error) {
			if command[0] == "a" {
				return bridge.Activate(ctx, first.Path())
			}
			secondStarted = true
			return bridge.Activate(ctx, second.Path())
		},
	})
	if err == nil {
		t.Fatal("handoff publication failure did not stop the replacement")
	}
	if secondStarted {
		t.Fatal("replacement ran before the handoff was retained")
	}
	task, readErr := store.Read("chain-task")
	if readErr != nil || task.Status != TaskInterventionRequired {
		t.Fatalf("failed handoff did not leave an actionable intervention: %+v, %v", task, readErr)
	}
}
