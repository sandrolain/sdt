//go:build unix

package cmd

import (
	"testing"

	"github.com/sandrolain/sdt/internal/delegation"
)

func TestRecordBrokerObservationPersistsRevisionCursorAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	store := delegation.NewStore(root)
	if err := store.Create(delegation.Task{ID: "observed", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("observed", "interrupted", "Worker exited", func(task *delegation.Task) error { task.Status = delegation.TaskInterventionRequired; return nil }); err != nil {
		t.Fatal(err)
	}
	identity := delegation.ProcessIdentity{PID: 1234, GroupID: 1234, StartToken: "0123456789abcdef0123456789abcdef"}
	if err := store.WriteBrokerState(delegation.BrokerState{Worker: &identity}); err != nil {
		t.Fatal(err)
	}
	if err := recordBrokerObservation(store, identity); err != nil {
		t.Fatal(err)
	}
	first, err := store.ReadBrokerState()
	if err != nil || first.Revisions["observed"] == 0 || first.Attention != 1 || first.ObservedAt.IsZero() {
		t.Fatalf("broker did not persist its observation: %+v, %v", first, err)
	}
	if err := recordBrokerObservation(store, identity); err != nil {
		t.Fatal(err)
	}
	second, err := store.ReadBrokerState()
	if err != nil || second.ObservedAt != first.ObservedAt || second.Attention != 1 {
		t.Fatalf("unchanged task produced a duplicate observation: %+v, %v", second, err)
	}
	if err := store.Update("observed", "principal_failed", "Closed after review", func(task *delegation.Task) error { task.Status = delegation.TaskFailed; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := recordBrokerObservation(store, identity); err != nil {
		t.Fatal(err)
	}
	third, err := store.ReadBrokerState()
	if err != nil || third.Attention != 0 || third.Revisions["observed"] <= second.Revisions["observed"] {
		t.Fatalf("broker did not advance its cursor after closure: %+v, %v", third, err)
	}
}

func TestAgentTaskBrokerCommandsAreRegistered(t *testing.T) {
	for _, path := range [][]string{{"agent", "task", "broker", "start"}, {"agent", "task", "broker", "stop"}, {"agent", "task", "broker", "status"}} {
		if _, _, err := rootCmd.Find(path); err != nil {
			t.Fatalf("command %v not registered: %v", path, err)
		}
	}
}

func TestBrokerReconcilesLostWorkerWithoutWaitClient(t *testing.T) {
	root := t.TempDir()
	store := delegation.NewStore(root)
	if err := store.Create(delegation.Task{ID: "lost-worker", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("lost-worker", "worker_started", "Worker started", func(task *delegation.Task) error {
		task.Status = delegation.TaskRunning
		task.Attempts = []delegation.Attempt{{ID: "attempt-1", Status: "running", ProcessID: 2147483000, ProcessStart: "0123456789abcdef0123456789abcdef"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	identity := delegation.ProcessIdentity{PID: 1234, GroupID: 1234, StartToken: "fedcba9876543210fedcba9876543210"}
	if err := store.WriteBrokerState(delegation.BrokerState{Worker: &identity}); err != nil {
		t.Fatal(err)
	}
	if err := recordBrokerObservation(store, identity); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("lost-worker")
	if err != nil || task.Status != delegation.TaskInterventionRequired {
		t.Fatalf("broker did not expose lost worker for recovery: %+v, %v", task, err)
	}
	state, err := store.ReadBrokerState()
	if err != nil || state.Attention != 1 || state.Revisions["lost-worker"] != task.Revision {
		t.Fatalf("broker cursor missed its recovery action: %+v, %v", state, err)
	}
}
