//go:build unix

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/sandrolain/sdt/internal/delegation"
	"strings"
	"testing"
	"time"
)

func TestPermissionDecisionAuthority(t *testing.T) {
	for _, tc := range []struct {
		source, rationale, mandate string
		valid                      bool
	}{
		{"principal", "bounded edit", "user request", true},
		{"user", "explicit response in conversation", "", true},
		{"principal", "bounded edit", "", false},
		{"principal", " ", "user request", false},
		{"policy", "rule", "rule", false},
		{"user", "", "", false},
	} {
		if err := validatePermissionAuthority(tc.source, tc.rationale, tc.mandate); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func TestPermissionHistoryFilters(t *testing.T) {
	records := []delegation.PermissionRecord{{RequestID: "a", Source: "principal", Disposition: "resolved"}, {RequestID: "b", Source: "user", Disposition: "pending", Escalation: "needs user"}}
	got := filterPermissionHistory(records, "user", "pending", true)
	if len(got) != 1 || got[0].RequestID != "b" {
		t.Fatalf("%+v", got)
	}
	if len(filterPermissionHistory(records, "principal", "pending", false)) != 0 {
		t.Fatal("filter combined as OR")
	}
	if len(filterPermissionHistory(records, "", "", false)) != 2 {
		t.Fatal("history lost")
	}
}

func TestDecodePermissionBatchInput(t *testing.T) {
	for _, raw := range []string{
		"decisions:\n  - request_id: request-a\n    option_id: allow-once\n  - request_id: request-b\n    option_id: reject-once\n",
		`{"decisions":[{"request_id":"request-a","option_id":"allow-once"}]}`,
	} {
		input, err := decodePermissionBatchInput(raw)
		if err != nil {
			t.Fatalf("decode %q: %v", raw, err)
		}
		if len(input.Decisions) == 0 || input.Decisions[0].RequestID != "request-a" || input.Decisions[0].OptionID != "allow-once" {
			t.Fatalf("decoded batch = %+v", input)
		}
	}
	for _, raw := range []string{"", "decisions: []", "decisions: ["} {
		if _, err := decodePermissionBatchInput(raw); err == nil {
			t.Fatalf("accepted invalid permission batch input %q", raw)
		}
	}
}

func TestResolvePermissionBatchCommandPersistsGroupedDecisions(t *testing.T) {
	root := researchTestProject(t)
	store := delegation.NewStore(root)
	if err := store.Create(delegation.Task{ID: "batch-cli", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("batch-cli", "attempt_started", "Attempt started", func(task *delegation.Task) error {
		task.Status = delegation.TaskRunning
		task.Attempts = []delegation.Attempt{{ID: "attempt-1", Agent: "reviewer", Status: "running", StartedAt: time.Now().UTC()}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := findProjectConfigAt()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := json.Marshal(cfg.Delegation.PermissionPolicy)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(policy)
	policyVersion := hex.EncodeToString(digest[:])
	for _, requestID := range []string{"request-a", "request-b"} {
		if err := store.RecordPermission("batch-cli", delegation.PermissionRecord{
			RequestID: requestID, AttemptID: "attempt-1", Capability: "file-write", Operation: "write",
			Scope: "file-write: write", PolicyVersion: policyVersion, Disposition: delegation.PermissionPending,
			Options: []delegation.PermissionChoice{{OptionID: "allow-once", Name: "Allow once"}, {OptionID: "reject-once", Name: "Reject once"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	input := []byte("decisions:\n  - request_id: request-a\n    option_id: allow-once\n  - request_id: request-b\n    option_id: reject-once\n")
	out := string(execute(t, agentTaskPermissionResolveBatchCmd, input,
		"batch-cli", "--source", "principal", "--mandate", "bounded change", "--rationale", "explicit choices"))
	if !strings.Contains(out, "Recorded 2 exact permission decisions") || !strings.Contains(out, "request-b") || !strings.Contains(out, "Reject once") {
		t.Fatalf("batch command summary = %q", out)
	}
	task, err := store.Read("batch-cli")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != delegation.TaskRunning || task.Permissions[0].OptionID != "allow-once" || task.Permissions[1].OptionID != "reject-once" {
		t.Fatalf("grouped decisions were not persisted: %+v", task)
	}
}
