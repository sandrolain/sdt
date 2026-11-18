//go:build unix

package delegation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitTestTask(t *testing.T, store *Store, id string, status TaskStatus) {
	t.Helper()
	if err := store.Create(Task{ID: id, Role: "backend"}); err != nil {
		t.Fatal(err)
	}
	if status == TaskQueued {
		return
	}
	if err := store.Update(id, "running", "fixture", func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if status == TaskRunning {
		return
	}
	if err := store.Update(id, "outcome", "fixture", func(task *Task) error { task.Status = status; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestWaitSnapshotModesAndInput(t *testing.T) {
	store := NewStore(t.TempDir())
	waitTestTask(t, store, "done", TaskFailed)
	waitTestTask(t, store, "ready", TaskResultReady)
	waitTestTask(t, store, "running", TaskRunning)
	waitTestTask(t, store, "cancelled", TaskCancelled)
	if err := store.Update("running", "permission", "fixture", func(task *Task) error {
		task.Permissions = []PermissionRecord{{RequestID: "permission-1", Disposition: PermissionPending}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode             string
		ids              []string
		satisfied        bool
		ready, remaining int
	}{
		{"one", []string{"ready"}, true, 1, 0},
		{"any", []string{"done", "ready", "running"}, true, 3, 0},
		{"all", []string{"done", "ready", "running"}, false, 3, 0},
		{"all", []string{"done"}, true, 1, 0},
		{"all", []string{"done", "cancelled"}, true, 2, 0},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		got, err := store.Wait(ctx, tc.ids, tc.mode)
		cancel()
		if err != nil || got.Satisfied != tc.satisfied || len(got.Ready) != tc.ready || len(got.Remaining) != tc.remaining {
			t.Fatalf("%s: %+v, %v", tc.mode, got, err)
		}
	}
	for _, ids := range [][]string{nil, {"done", "done"}, {"../escape"}, {"absent"}} {
		if _, err := store.Wait(context.Background(), ids, "any"); err == nil {
			t.Fatalf("accepted ids %v", ids)
		}
	}
}

func TestWaitWorkerUnreachable(t *testing.T) {
	store := NewStore(t.TempDir())
	waitTestTask(t, store, "running", TaskRunning)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := store.Wait(ctx, []string{"running"}, "one")
	if err == nil || got.Reason != "worker_unreachable" || got.Satisfied {
		t.Fatalf("%+v, %v", got, err)
	}
}

func TestWaitTimeoutDoesNotCancelTask(t *testing.T) {
	store := NewStore(t.TempDir())
	waitTestTask(t, store, "running", TaskRunning)
	before, _ := store.Read("running")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result, err := store.Wait(ctx, []string{"running"}, "one")
	if !errors.Is(err, context.DeadlineExceeded) || !result.TimedOut || result.Satisfied {
		t.Fatalf("%+v, %v", result, err)
	}
	after, _ := store.Read("running")
	if after.Status != before.Status || after.Revision != before.Revision {
		t.Fatal("waiting mutated task")
	}
}

func TestWaitReceivesCrossProcessPublication(t *testing.T) {
	for _, noPublish := range []bool{false, true} {
		t.Run(fmt.Sprint(noPublish), func(t *testing.T) { testWaitPublication(t, noPublish) })
	}
}

func testWaitPublication(t *testing.T, noPublish bool) {
	if noPublish {
		t.Setenv("SDT_TEST_NOTIFIER_NO_PUBLISH", "1")
	}
	root := shortProjectTemp(t)
	store := NewStore(root)
	waitTestTask(t, store, "task-1", TaskQueued)
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1}); err != nil || !admitted {
		t.Fatalf("admission: %v %v", admitted, err)
	}
	token, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptStarting("task-1", Attempt{ID: "attempt-1"}, token); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SDT_TEST_NOTIFIER_ROOT", root)
	for key, name := range map[string]string{"READY": "ready", "GO": "go", "SEEN": "seen", "ERROR": "error"} {
		t.Setenv("SDT_TEST_NOTIFIER_"+key, filepath.Join(root, name))
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := StartWorkerWithToken(executable, root, filepath.Join(root, "worker.log"), token, "-test.run=^TestNotifierWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	if !waitForFile(filepath.Join(root, "ready"), 5*time.Second) {
		t.Fatal("notifier did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan WaitSnapshot, 1)
	errorsCh := make(chan error, 1)
	go func() {
		snapshot, err := store.Wait(ctx, []string{"task-1"}, "one")
		result <- snapshot
		errorsCh <- err
	}()
	if err := os.WriteFile(filepath.Join(root, "go"), []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := <-result
	if err := <-errorsCh; err != nil || got.Reason != "intervention_required" || !got.Satisfied {
		t.Fatalf("%+v, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "seen"), []byte("seen"), 0o600); err != nil {
		t.Fatal(err)
	}
}
