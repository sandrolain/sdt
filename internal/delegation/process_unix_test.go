//go:build unix

package delegation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

func TestWorkerHelperProcess(t *testing.T) {
	if !hasWorkerToken(os.Args) {
		return
	}
	if os.Getenv("SDT_TEST_NOTIFIER_ROOT") != "" {
		notifierWorkerProcess()
		os.Exit(0)
	}
	child := exec.Command("sleep", "60")
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	if path := os.Getenv("SDT_TEST_WORKER_CHILD_PIDFILE"); path != "" {
		_ = os.WriteFile(path, []byte(strconv.Itoa(child.Process.Pid)), 0o600)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestNotifierWorkerHelperProcess(t *testing.T) {
	if !hasWorkerToken(os.Args) {
		return
	}
	notifierWorkerProcess()
	os.Exit(0)
}

func notifierWorkerProcess() {
	root := os.Getenv("SDT_TEST_NOTIFIER_ROOT")
	store := NewStore(root)
	identity := ProcessIdentity{PID: os.Getpid(), GroupID: os.Getpid(), StartToken: workerToken(os.Args)}
	if err := store.RecordAttemptWorker("task-1", "attempt-1", identity); err != nil {
		notifierWorkerError("record: " + err.Error())
	}
	if err := store.AttachWorker("task-1", identity); err != nil {
		notifierWorkerError("capacity: " + err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	notifier, err := store.Listen(ctx, "task-1", identity)
	if err != nil {
		notifierWorkerError("listen: " + err.Error())
	}
	if err := os.WriteFile(os.Getenv("SDT_TEST_NOTIFIER_READY"), []byte("ready"), 0o600); err != nil {
		notifierWorkerError("ready: " + err.Error())
	}
	if !waitForFile(os.Getenv("SDT_TEST_NOTIFIER_GO"), 5*time.Second) {
		notifierWorkerError("go timeout")
	}
	if resultPath := os.Getenv("SDT_TEST_NOTIFIER_PERMISSION_RESULT"); resultPath != "" {
		options := ACPWorkerOptions{Store: store, Notifier: notifier, TaskID: "task-1", AttemptID: "attempt-1", Agent: "reviewer", Model: "test"}
		optionID, err := options.handlePermission(context.Background(), acpbridge.Permission{
			RequestID: "acp-perm-pending-1", ToolCallID: "call-1", ToolKind: "edit", Title: "write file",
			Options: []acpbridge.PermissionOption{
				{OptionID: "allow-once", Kind: "allow_once", Name: "Allow once"},
				{OptionID: "reject-once", Kind: "reject_once", Name: "Reject once"},
			},
		})
		if err != nil {
			notifierWorkerError("permission wait: " + err.Error())
		}
		if err := os.WriteFile(resultPath, []byte(optionID), 0o600); err != nil {
			notifierWorkerError("permission result: " + err.Error())
		}
		_ = notifier.Close()
		cancel()
		return
	}
	if err := notifier.Update("permission_pending", "Permission needs intervention", func(task *Task) error {
		task.Status = TaskInterventionRequired
		return nil
	}); err != nil {
		notifierWorkerError("update: " + err.Error())
	}
	_, err = store.Read("task-1")
	if err != nil {
		notifierWorkerError("read: " + err.Error())
	}
	if !waitForFile(os.Getenv("SDT_TEST_NOTIFIER_SEEN"), 5*time.Second) {
		notifierWorkerError("seen timeout")
	}
	_ = notifier.Close()
	cancel()
}

func notifierWorkerError(message string) {
	_ = os.WriteFile(os.Getenv("SDT_TEST_NOTIFIER_ERROR"), []byte(message), 0o600)
	os.Exit(1)
}

func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func workerToken(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--sdt-delegation-worker=") {
			return strings.TrimPrefix(arg, "--sdt-delegation-worker=")
		}
	}
	return ""
}

func TestStartAndCancelWorkerProcessGroup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "worker.log")
	t.Setenv("SDT_TEST_WORKER_CHILD_PIDFILE", filepath.Join(dir, "child.pid"))
	identity, err := StartWorker(executable, dir, logPath, "-test.run=^TestWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	deadline := time.Now().Add(3 * time.Second)
	for !WorkerMatches(identity) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !WorkerMatches(identity) {
		t.Fatal("started worker does not match its persisted identity")
	}
	childPath := filepath.Join(dir, "child.pid")
	childDeadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(childPath); err == nil || time.Now().After(childDeadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	childPID, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatalf("worker did not start its child process: %v", err)
	}
	store := NewStore(dir)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1, "group": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	if err := store.AttachWorker("task-1", identity); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("task-1", "started", "Worker started", func(task *Task) error {
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "starting"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptWorker("task-1", "attempt-1", identity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Cancel(ctx, "task-1"); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("task-1")
	if err != nil || task.Status != TaskCancelled {
		t.Fatalf("cancelled task = %+v, error = %v", task, err)
	}
	if WorkerMatches(identity) {
		t.Fatal("worker identity still matches after group stop")
	}
	childGone := false
	childDeadline = time.Now().Add(time.Second)
	for time.Now().Before(childDeadline) {
		args, _ := exec.Command("ps", "-p", strings.TrimSpace(string(childPID)), "-o", "args=").Output()
		if !strings.Contains(string(args), "sleep 60") {
			childGone = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !childGone {
		t.Fatal("worker child remained after process-group cancellation")
	}
	if err := SignalWorkerGroup(identity, 0); err != ErrStaleWorker {
		t.Fatalf("stale process identity error = %v, want ErrStaleWorker", err)
	}
}

func TestReconcileAfterWorkerDeathKeepsWorkspaceEvidence(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "worker.log")
	t.Setenv("SDT_TEST_WORKER_CHILD_PIDFILE", filepath.Join(dir, "child.pid"))
	identity, err := StartWorker(executable, dir, logPath, "-test.run=^TestWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	deadline := time.Now().Add(3 * time.Second)
	for !WorkerMatches(identity) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !WorkerMatches(identity) {
		t.Fatal("started worker does not match its persisted identity")
	}
	store := NewStore(dir)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1, "group": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	if err := store.AttachWorker("task-1", identity); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("task-1", "started", "Worker started", func(task *Task) error {
		task.Artifacts = []Artifact{{Name: "partial", Path: "artifacts/partial.md"}}
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "starting"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptWorker("task-1", "attempt-1", identity); err != nil {
		t.Fatal(err)
	}
	if err := SignalWorkerGroup(identity, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deathDeadline := time.Now().Add(3 * time.Second)
	for WorkerMatches(identity) && time.Now().Before(deathDeadline) {
		time.Sleep(20 * time.Millisecond)
	}
	task, err := store.Reconcile("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskInterventionRequired || len(task.Artifacts) != 1 || task.Artifacts[0].Path != "artifacts/partial.md" {
		t.Fatalf("worker death reconciliation lost task/evidence: %+v", task)
	}
	released, err := store.ReconcileCapacity(time.Now())
	if err != nil || len(released) != 0 {
		t.Fatalf("dead worker capacity reconciliation = %v, %v", released, err)
	}
	if err := store.Create(Task{ID: "task-2", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-2", map[string]int{"entry": 1, "group": 1}); err != nil || !admitted {
		t.Fatalf("recovered capacity was not released: %v, %v", admitted, err)
	}
}

func TestReconcileFindsWorkerStartedBeforePIDWasPersisted(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	token, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptStarting("task-1", Attempt{ID: "attempt-1"}, token); err != nil {
		t.Fatal(err)
	}
	identity, err := StartWorkerWithToken(executable, dir, filepath.Join(dir, "worker.log"), token, "-test.run=^TestWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	deadline := time.Now().Add(3 * time.Second)
	for !WorkerMatches(identity) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	task, err := store.Reconcile("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || len(task.Attempts) != 1 || task.Attempts[0].ProcessID != identity.PID {
		t.Fatalf("startup reconciliation did not recover worker identity: %+v", task)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Cancel(ctx, "task-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSubscribeReadRecheckAcrossWorkerProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := shortProjectTemp(t)
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	token, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptStarting("task-1", Attempt{ID: "attempt-1"}, token); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SDT_TEST_NOTIFIER_ROOT", root)
	ready := filepath.Join(root, "notifier.ready")
	goFile := filepath.Join(root, "notifier.go")
	seen := filepath.Join(root, "notifier.seen")
	errorFile := filepath.Join(root, "notifier.error")
	t.Setenv("SDT_TEST_NOTIFIER_READY", ready)
	t.Setenv("SDT_TEST_NOTIFIER_GO", goFile)
	t.Setenv("SDT_TEST_NOTIFIER_SEEN", seen)
	t.Setenv("SDT_TEST_NOTIFIER_ERROR", errorFile)
	identity, err := StartWorkerWithToken(executable, root, filepath.Join(root, "worker.log"), token, "-test.run=^TestNotifierWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	if !waitForFile(ready, 5*time.Second) {
		log, _ := os.ReadFile(filepath.Join(root, "worker.log"))
		helperError, _ := os.ReadFile(errorFile)
		t.Fatalf("worker notifier did not become ready (worker_alive=%v): %s %s", WorkerMatches(identity), log, helperError)
	}
	subscription, err := store.Subscribe(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Close() //nolint:errcheck
	initial, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if initial.Revision < subscription.InitialRevision {
		t.Fatalf("state revision %d precedes subscription ack %d", initial.Revision, subscription.InitialRevision)
	}
	if err := os.WriteFile(goFile, []byte("publish"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case revision, ok := <-subscription.Revisions:
		if !ok {
			t.Fatal("worker disconnected before publishing its transition")
		}
		state, err := store.Read("task-1")
		if err != nil {
			t.Fatal(err)
		}
		if revision < state.Revision || state.Status != TaskInterventionRequired {
			t.Fatalf("notification/state = %d/%+v", revision, state)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the durable task transition")
	}
	if err := os.WriteFile(seen, []byte("seen"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionWaitResolvesAcrossWorkerProcessAfterDurableWrite(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := shortProjectTemp(t)
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	token, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptStarting("task-1", Attempt{ID: "attempt-1", Agent: "reviewer", Model: "test"}, token); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(root, "permission.ready")
	goFile := filepath.Join(root, "permission.go")
	errorFile := filepath.Join(root, "permission.error")
	resultFile := filepath.Join(root, "permission.result")
	t.Setenv("SDT_TEST_NOTIFIER_ROOT", root)
	t.Setenv("SDT_TEST_NOTIFIER_READY", ready)
	t.Setenv("SDT_TEST_NOTIFIER_GO", goFile)
	t.Setenv("SDT_TEST_NOTIFIER_SEEN", filepath.Join(root, "permission.seen"))
	t.Setenv("SDT_TEST_NOTIFIER_ERROR", errorFile)
	t.Setenv("SDT_TEST_NOTIFIER_PERMISSION_RESULT", resultFile)
	identity, err := StartWorkerWithToken(executable, root, filepath.Join(root, "worker.log"), token, "-test.run=^TestNotifierWorkerHelperProcess$")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupWorker(t, identity) })
	if !waitForFile(ready, 5*time.Second) {
		t.Fatalf("permission worker did not start: %s", readIfExists(errorFile))
	}
	if err := os.WriteFile(goFile, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		task, err := store.Read("task-1")
		if err != nil {
			t.Fatal(err)
		}
		if task.Status == TaskInterventionRequired && len(task.Permissions) == 1 {
			if task.Permissions[0].Disposition != "pending" || task.Permissions[0].AttemptID != "attempt-1" {
				t.Fatalf("permission was not durably pending for the current attempt: %+v", task.Permissions[0])
			}
			break
		}
		if helperError := readIfExists(errorFile); helperError != "" {
			t.Fatalf("worker permission handler failed: %s", helperError)
		}
		if time.Now().After(deadline) {
			t.Fatalf("permission did not become actionable: %+v", task)
		}
		time.Sleep(20 * time.Millisecond)
	}
	resolveErr := store.ResolvePermissionAndWake("task-1", "attempt-1", "acp-perm-pending-1", "reject-once", "principal", "within mandate", "", "")
	if resolveErr != nil && !waitForFile(resultFile, 5*time.Second) {
		t.Fatalf("resolution was persisted but neither worker notification nor state reread completed: %v", resolveErr)
	}
	if !waitForFile(resultFile, 5*time.Second) {
		t.Fatalf("worker did not consume the durable permission answer: %s", readIfExists(errorFile))
	}
	result, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "reject-once" {
		t.Fatalf("worker selected %q, want the exact rejected option", result)
	}
	task, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || task.Permissions[0].Disposition != "resolved" || task.Permissions[0].Source != "principal" || task.Permissions[0].OptionID != "reject-once" {
		t.Fatalf("durable decision was not retained: %+v", task)
	}
}

func readIfExists(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func shortProjectTemp(t *testing.T) string {
	t.Helper()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Clean(filepath.Join(workingDir, "..", ".."))
	tmpRoot := filepath.Join(projectRoot, "context", "tmp")
	if err := os.MkdirAll(tmpRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(tmpRoot, "d-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func cleanupWorker(t *testing.T, identity ProcessIdentity) {
	t.Helper()
	if WorkerMatches(identity) {
		_ = syscall.Kill(-identity.GroupID, syscall.SIGKILL)
	}
}

func hasWorkerToken(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--sdt-delegation-worker=") {
			return true
		}
	}
	return false
}
