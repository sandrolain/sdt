package delegation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const storeHelperEnv = "SDT_DELEGATION_STORE_HELPER"

func TestStoreCreateUpdateAndRejectsInvalidIDs(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskQueued || task.Revision != 1 || task.SchemaVersion != stateSchemaVersion {
		t.Fatalf("created task has wrong initial state: %+v", task)
	}
	if err := store.Update("task-1", "started", "Worker started", func(task *Task) error {
		task.Status = TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	task, err = store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || task.Revision != 2 || len(task.Events) != 2 || task.Events[1].ID != 2 {
		t.Fatalf("updated task has wrong revision/event: %+v", task)
	}
	if err := store.Create(Task{ID: "../escape"}); err == nil {
		t.Fatal("expected unsafe task id to fail")
	}
}

func TestPrincipalAcceptanceAndCorrectionAreRecordedSeparately(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "decision-task", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("decision-task", "started", "Worker started", func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("decision-task", "result_ready", "Result delivered", func(task *Task) error { task.Status = TaskResultReady; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptResult("decision-task", "Reviewed and accepted"); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("decision-task")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskAccepted || task.PrincipalDecision == nil || task.PrincipalDecision.AcceptedAt.IsZero() {
		t.Fatalf("acceptance was not recorded: %+v", task)
	}
	if err := store.RequestCorrection("decision-task", "Address the missing edge case"); err != nil {
		t.Fatal(err)
	}
	task, err = store.Read("decision-task")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskQueued || task.Correction != "Address the missing edge case" || task.PrincipalDecision == nil || task.PrincipalDecision.CorrectedAt.IsZero() || len(task.PrincipalDecisions) != 2 || task.PrincipalDecisions[0].Action != "accepted" {
		t.Fatalf("correction was not independently recorded: %+v", task)
	}
}

func TestResolvePermissionWithPolicyAcceptsCurrentAndInvalidatesStaleRequests(t *testing.T) {
	newPending := func(t *testing.T, id string) *Store {
		t.Helper()
		store := NewStore(t.TempDir())
		if err := store.Create(Task{ID: id, Role: "reviewer"}); err != nil {
			t.Fatal(err)
		}
		if err := store.Update(id, "attempt_started", "Attempt started", func(task *Task) error {
			task.Status = TaskRunning
			task.Attempts = []Attempt{{ID: "attempt-1", Agent: "reviewer", Status: "running"}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordPermission(id, PermissionRecord{
			RequestID: "request-1", AttemptID: "attempt-1", Capability: "file-write",
			Options: []PermissionChoice{{OptionID: "allow", Name: "Allow"}}, PolicyVersion: "policy-v1", Disposition: PermissionPending,
		}); err != nil {
			t.Fatal(err)
		}
		return store
	}

	accepted := newPending(t, "permission-current")
	if err := accepted.ResolvePermissionWithPolicy("permission-current", "attempt-1", "request-1", "allow", "principal", "within mandate", "policy-v1", "rule-1", "review approved"); err != nil {
		t.Fatal(err)
	}
	resolved, err := accepted.Read("permission-current")
	if err != nil || resolved.Permissions[0].Disposition != "resolved" || resolved.Permissions[0].Mandate != "review approved" || resolved.Status != TaskRunning {
		t.Fatalf("current-policy decision = %+v, %v", resolved, err)
	}

	stale := newPending(t, "permission-stale")
	if err := stale.ResolvePermissionWithPolicy("permission-stale", "attempt-1", "request-1", "allow", "principal", "obsolete", "policy-v2", "rule-2", "review approved"); err == nil {
		t.Fatal("accepted a request against a changed policy")
	}
	invalidated, err := stale.Read("permission-stale")
	if err != nil || invalidated.Permissions[0].Disposition != "stale" || invalidated.Permissions[0].PolicyRule != "invalidated; current policy version policy-v2" {
		t.Fatalf("stale permission was not recorded: %+v, %v", invalidated, err)
	}
	if err := stale.ResolvePermissionWithPolicy("permission-stale", "attempt-1", "missing", "allow", "principal", "", "policy-v2", "rule-2"); err == nil {
		t.Fatal("accepted a request that is not outstanding")
	}
}

func TestStoreReconcileRetainsArtifactsAndMarksLostWorker(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "task-1", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("task-1", "started", "Worker started", func(task *Task) error {
		task.Status = TaskRunning
		task.Artifacts = []Artifact{{Name: "partial", Path: "artifacts/partial.md"}}
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "running", ProcessID: 2147483000, ProcessStart: "missing-process-token"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	task, err := store.Reconcile("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskInterventionRequired || task.Attempts[0].Status != "interrupted" {
		t.Fatalf("lost worker was not reconciled: %+v", task)
	}
	if len(task.Artifacts) != 1 || task.Artifacts[0].Path != "artifacts/partial.md" {
		t.Fatalf("partial artifact was not retained: %+v", task.Artifacts)
	}
}

func TestStoreIgnoresAbandonedPartialStatePublication(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "task-1", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(store.Root(), "tasks", "task-1", ".state-crashed")
	if err := os.WriteFile(partial, []byte(`{"schema_version":`), 0o600); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskQueued || task.Revision != 1 {
		t.Fatalf("abandoned temp file replaced the published state: %+v", task)
	}
}

func TestStoreListAndReconcileCancellationReleaseCapacity(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"task-a", "task-b"} {
		if err := store.Create(Task{ID: id, Role: "reviewer"}); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := store.List()
	if err != nil || len(tasks) != 2 {
		t.Fatalf("task list = %d records, %v", len(tasks), err)
	}
	if admitted, err := store.TryAdmit("task-a", map[string]int{"agent": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	if err := store.Update("task-a", "cancelling", "cancel requested", func(task *Task) error {
		task.Status = TaskCancelling
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	result, err := store.Reconcile("task-a")
	if err != nil || result.Status != TaskCancelled {
		t.Fatalf("cancel reconciliation = %+v, %v", result, err)
	}
	if admitted, err := store.TryAdmit("task-b", map[string]int{"agent": 1}); err != nil || !admitted {
		t.Fatalf("cancelled task retained its capacity: %v, %v", admitted, err)
	}
}

func TestReconcileCapacityReleasesExpiredAndLostWorkers(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"expired", "lost-worker", "unstarted"} {
		if err := store.Create(Task{ID: id, Role: "reviewer"}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if err := store.writeCapacity(CapacityState{Reservations: []CapacityReservation{
		{TaskID: "expired", Keys: []string{"a"}, Deadline: now.Add(-time.Second)},
		{TaskID: "lost-worker", Keys: []string{"b"}, Worker: ProcessIdentity{PID: 2147483000, GroupID: 2147483000, StartToken: "missing-token"}},
		{TaskID: "unstarted", Keys: []string{"c"}, Deadline: now.Add(time.Hour)},
	}}); err != nil {
		t.Fatal(err)
	}
	released, err := store.ReconcileCapacity(now)
	if err != nil || !containsString(released, "expired") || !containsString(released, "lost-worker") || containsString(released, "unstarted") {
		t.Fatalf("reconciled capacity = %v, %v", released, err)
	}
	if !equalStrings([]string{"a", "b"}, []string{"a", "b"}) || equalStrings([]string{"a"}, []string{"b"}) || equalStrings([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("capacity key equality returned an incorrect result")
	}
}

func TestStoreLockHelperProcess(t *testing.T) {
	if os.Getenv(storeHelperEnv) == "" {
		return
	}
	store := NewStore(os.Getenv(storeHelperEnv))
	if os.Getenv(storeHelperEnv+"_admit") == "1" {
		taskID := os.Getenv(storeHelperEnv + "_task")
		admitted, err := store.TryAdmit(taskID, map[string]int{"entry": 1, "group": 1})
		if err != nil {
			os.Exit(4)
		}
		result := "queued"
		if admitted {
			result = "admitted"
		}
		if err := os.WriteFile(os.Getenv(storeHelperEnv+"_result"), []byte(result), 0o600); err != nil {
			os.Exit(5)
		}
		os.Exit(0)
	}
	if os.Getenv(storeHelperEnv+"_exit") == "1" {
		_ = store.WithLock("task-task-1", func() error { os.Exit(0); return nil })
		os.Exit(2)
	}
	count, _ := strconv.Atoi(os.Getenv(storeHelperEnv + "_count"))
	for i := 0; i < count; i++ {
		if err := store.Update("task-1", "tick", "Increment", func(task *Task) error {
			task.Status = TaskRunning
			return nil
		}); err != nil {
			os.Exit(3)
		}
	}
	os.Exit(0)
}

func TestTryAdmitReservesAllCapacityKeysAtomically(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"task-a", "task-b", "task-c"} {
		if err := store.Create(Task{ID: id, Role: "reviewer"}); err != nil {
			t.Fatal(err)
		}
	}
	limits := map[string]int{"entry-a": 1, "group-shared": 1}
	if admitted, err := store.TryAdmit("task-a", limits); err != nil || !admitted {
		t.Fatalf("first admission = %v, %v", admitted, err)
	}
	if admitted, err := store.TryAdmit("task-b", map[string]int{"entry-b": 1, "group-shared": 1}); err != nil || admitted {
		t.Fatalf("shared group should queue the second task: %v, %v", admitted, err)
	}
	if admitted, err := store.TryAdmit("task-c", map[string]int{"entry-b": 1}); err != nil || !admitted {
		t.Fatalf("failed admission must not retain a partial entry reservation: %v, %v", admitted, err)
	}
	if err := store.ReleaseCapacity("task-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseCapacity("task-c"); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-b", map[string]int{"entry-b": 1, "group-shared": 1}); err != nil || !admitted {
		t.Fatalf("released group capacity was not reusable: %v, %v", admitted, err)
	}
}

func TestTryAdmitSerializesAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	for _, id := range []string{"task-a", "task-b"} {
		if err := store.Create(Task{ID: id, Role: "reviewer"}); err != nil {
			t.Fatal(err)
		}
	}
	commands := make([]*exec.Cmd, 0, 2)
	for _, id := range []string{"task-a", "task-b"} {
		result := filepath.Join(root, id+".result")
		cmd := exec.Command(os.Args[0], "-test.run=^TestStoreLockHelperProcess$")
		cmd.Env = append(os.Environ(), storeHelperEnv+"="+root, storeHelperEnv+"_admit=1", storeHelperEnv+"_task="+id, storeHelperEnv+"_result="+result)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	admitted, queued := 0, 0
	for _, id := range []string{"task-a", "task-b"} {
		result, err := os.ReadFile(filepath.Join(root, id+".result"))
		if err != nil {
			t.Fatal(err)
		}
		switch string(result) {
		case "admitted":
			admitted++
		case "queued":
			queued++
		default:
			t.Fatalf("unexpected admission result %q", result)
		}
	}
	if admitted != 1 || queued != 1 {
		t.Fatalf("cross-process capacity results admitted=%d queued=%d", admitted, queued)
	}
}

func TestStoreSerializesCrossProcessUpdates(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	const workers, updates = 4, 8
	commands := make([]*exec.Cmd, 0, workers)
	for i := 0; i < workers; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestStoreLockHelperProcess$")
		cmd.Env = append(os.Environ(), storeHelperEnv+"="+root, storeHelperEnv+"_count="+strconv.Itoa(updates))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	want := uint64(1 + workers*updates)
	if task.Revision != want || uint64(len(task.Events)) != want {
		t.Fatalf("revision/events = %d/%d, want %d/%d", task.Revision, len(task.Events), want, want)
	}
}

func TestStoreLockReleasesWhenOwnerDies(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStoreLockHelperProcess$")
	cmd.Env = append(os.Environ(), storeHelperEnv+"="+root, storeHelperEnv+"_exit=1")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("task-1", "recovered", "Lock released after owner exit", func(task *Task) error {
		task.Status = TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
