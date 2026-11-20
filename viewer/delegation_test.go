package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/sandrolain/sdt/internal/delegation"
)

func TestDelegationTasksExposeTheSameDurableInbox(t *testing.T) {
	root := t.TempDir()
	store := delegation.NewStore(root)
	if err := store.Create(delegation.Task{ID: "viewer-inbox", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("viewer-inbox", "started", "Worker started", func(task *delegation.Task) error {
		task.Status = delegation.TaskRunning
		task.Attempts = []delegation.Attempt{{ID: "attempt-1", Status: "running"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPermission("viewer-inbox", delegation.PermissionRecord{
		RequestID: "permission-1", AttemptID: "attempt-1", Capability: "file-write",
		Options: []delegation.PermissionChoice{{OptionID: "allow", Name: "Allow"}}, Disposition: delegation.PermissionPending,
	}); err != nil {
		t.Fatal(err)
	}
	server := &server{root: root, delegationReadOnly: true}
	request := httptest.NewRequest("GET", "http://localhost/api/delegation/tasks", nil)
	request.Header.Set("Origin", "http://localhost")
	response := httptest.NewRecorder()
	server.handleDelegationTasks(response, request)
	if response.Code != 200 {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var payload delegatedTasksResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Inbox) != 1 || payload.Inbox[0].TaskID != "viewer-inbox" || len(payload.Inbox[0].PendingRequestIDs) != 1 || payload.Inbox[0].PendingRequestIDs[0] != "permission-1" {
		t.Fatalf("viewer inbox did not mirror durable task state: %+v", payload.Inbox)
	}
}
