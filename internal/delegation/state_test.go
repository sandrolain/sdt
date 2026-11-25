package delegation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const storeHelperEnv = "SDT_DELEGATION_STORE_HELPER"

func TestBrokerStatePersistence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store := NewStore(root)
	initial, err := store.ReadBrokerState()
	if err != nil || initial.Worker != nil || initial.Attention != 0 || len(initial.Revisions) != 0 {
		t.Fatalf("missing state: %+v, %v", initial, err)
	}
	want := BrokerState{Attention: 2, Revisions: map[string]uint64{"task-1": 3}}
	if err := store.WriteBrokerState(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(root).ReadBrokerState()
	if err != nil || got.Attention != 2 || len(got.Revisions) != 1 || got.Revisions["task-1"] != 3 {
		t.Fatalf("persisted state: %+v, %v", got, err)
	}
	path := filepath.Join(store.root, "broker.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadBrokerState(); err == nil || !strings.Contains(err.Error(), "decode broker state") {
		t.Fatalf("corrupt state error: %v", err)
	}
}

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
	store, _, _, _ := acceptedIntegrationWorkspace(t, "decision-task")
	task, err := store.Read("decision-task")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskAccepted || task.PrincipalDecision == nil || task.PrincipalDecision.AcceptedAt.IsZero() || task.PrincipalDecision.CandidateDigest == "" {
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

func TestPrincipalDispositionWaitsForDeclaredChecksToSettle(t *testing.T) {
	task := Task{VerificationConfig: []VerificationCheckConfig{{Name: "unit", Run: "go test ./..."}}}
	if err := verificationReadyForPrincipal(task); err == nil {
		t.Fatal("principal disposition was allowed before declared checks started")
	}
	task.Verification = &VerificationRun{Status: VerificationRunning}
	if err := verificationReadyForPrincipal(task); err == nil {
		t.Fatal("principal disposition was allowed while declared checks were running")
	}
	task.Verification.Status = VerificationFailed
	if err := verificationReadyForPrincipal(task); err != nil {
		t.Fatalf("settled failure should be available for principal review: %v", err)
	}
}

func TestPrincipalCanFailInterruptedTaskAndDiscardReadyResult(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "recover-failed", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("recover-failed", "interrupted", "Worker was lost", func(task *Task) error {
		task.Status = TaskInterventionRequired
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "interrupted", Error: "worker exited"}}
		task.Artifacts = []Artifact{{Name: "partial", Path: "artifacts/partial.md"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Fail("recover-failed", "operator stopped recovery"); err != nil {
		t.Fatal(err)
	}
	failed, err := store.Read("recover-failed")
	if err != nil || failed.Status != TaskFailed || failed.Attempts[0].Error != "operator stopped recovery" || len(failed.Artifacts) != 1 || failed.Events[len(failed.Events)-1].Type != "principal_failed" {
		t.Fatalf("failed task did not retain durable evidence: %+v, %v", failed, err)
	}

	if err := store.Create(Task{ID: "recover-discard", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("recover-discard", "started", "Worker started", func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("recover-discard", "result_ready", "Result delivered", func(task *Task) error {
		task.Status = TaskResultReady
		task.Artifacts = []Artifact{{Name: "result", Path: "artifacts/result.md"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DiscardResult("recover-discard", "not needed"); err != nil {
		t.Fatal(err)
	}
	discarded, err := store.Read("recover-discard")
	if err != nil || discarded.Status != TaskDiscarded || discarded.PrincipalDecision == nil || discarded.PrincipalDecision.Action != "discarded" || discarded.PrincipalDecision.DiscardedAt.IsZero() || len(discarded.Artifacts) != 1 {
		t.Fatalf("discard was not retained: %+v, %v", discarded, err)
	}
	if err := store.DiscardResult("recover-discard", "again"); err == nil {
		t.Fatal("accepted a repeated terminal disposition")
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

func createPendingPermissionBatch(t *testing.T, id string) (*Store, uint64) {
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
	for _, requestID := range []string{"request-a", "request-b"} {
		if err := store.RecordPermission(id, PermissionRecord{
			RequestID: requestID, AttemptID: "attempt-1", Capability: "file-write",
			Options:       []PermissionChoice{{OptionID: "allow", Name: "Allow"}, {OptionID: "reject", Name: "Reject"}},
			PolicyVersion: "policy-v1", Disposition: PermissionPending,
		}); err != nil {
			t.Fatal(err)
		}
	}
	task, err := store.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	return store, task.Revision
}

func TestPermissionBatchPersistsMixedChoicesAndIndividualEvents(t *testing.T) {
	store, before := createPendingPermissionBatch(t, "batch-permissions")
	if err := store.ResolvePermissionBatchWithPolicy("batch-permissions", "attempt-1", []PermissionBatchDecision{
		{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-b", OptionID: "reject"},
	}, PermissionSourcePrincipal, "bounded task mandate", "policy-v1", "explicit CLI batch decision", "task mandate"); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("batch-permissions")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskRunning || task.Revision != before+2 || len(task.Events) < 2 {
		t.Fatalf("batch did not advance state once per decision: status=%s revision=%d events=%d", task.Status, task.Revision, len(task.Events))
	}
	for i, want := range []string{"allow", "reject"} {
		record := task.Permissions[i]
		if record.Disposition != PermissionResolved || record.OptionID != want || record.Source != PermissionSourcePrincipal || record.Rationale != "bounded task mandate" || record.Mandate != "task mandate" {
			t.Fatalf("decision %d lost exact choice or provenance: %+v", i, record)
		}
		if got := task.Events[len(task.Events)-2+i]; got.Type != "permission_resolved" || !strings.Contains(got.Summary, record.RequestID) {
			t.Fatalf("decision %d has no matching event: %+v", i, got)
		}
	}
	if task.Status == TaskFailed || task.Status == TaskCancelled {
		t.Fatalf("an offered reject choice closed the task: %s", task.Status)
	}
}

func TestPermissionBatchRejectsInvalidInputAtomically(t *testing.T) {
	tests := []struct {
		name      string
		decisions []PermissionBatchDecision
		version   string
	}{
		{name: "not offered", decisions: []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-b", OptionID: "not-offered"}}, version: "policy-v1"},
		{name: "changed policy", decisions: []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-b", OptionID: "allow"}}, version: "policy-v2"},
		{name: "unknown request", decisions: []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}, {RequestID: "missing", OptionID: "allow"}}, version: "policy-v1"},
		{name: "duplicate request", decisions: []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-a", OptionID: "reject"}}, version: "policy-v1"},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := fmt.Sprintf("batch-invalid-%d", i)
			store, before := createPendingPermissionBatch(t, id)
			err := store.ResolvePermissionBatchWithPolicy(id, "attempt-1", tc.decisions, PermissionSourcePrincipal, "bounded task mandate", tc.version, "explicit CLI batch decision", "task mandate")
			if err == nil {
				t.Fatal("invalid batch was accepted")
			}
			task, readErr := store.Read(id)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if task.Revision != before || task.Status != TaskInterventionRequired || task.Permissions[0].Disposition != PermissionPending || task.Permissions[1].Disposition != PermissionPending {
				t.Fatalf("invalid batch partially changed durable state: %+v", task)
			}
		})
	}
}

func TestPermissionBatchRequiresMandateAndKeepsRemainingRequestsActionable(t *testing.T) {
	t.Run("principal mandate required", func(t *testing.T) {
		store, before := createPendingPermissionBatch(t, "batch-authority")
		err := store.ResolvePermissionBatchWithPolicy("batch-authority", "attempt-1", []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}}, PermissionSourcePrincipal, "bounded task mandate", "policy-v1", "explicit CLI batch decision", "")
		if err == nil {
			t.Fatal("accepted a principal batch without a mandate reference")
		}
		task, err := store.Read("batch-authority")
		if err != nil || task.Revision != before || task.Permissions[0].Disposition != PermissionPending {
			t.Fatalf("authority validation mutated state: %+v, %v", task, err)
		}
	})
	t.Run("remaining request preserves intervention", func(t *testing.T) {
		store, _ := createPendingPermissionBatch(t, "batch-partial")
		if err := store.ResolvePermissionBatchWithPolicy("batch-partial", "attempt-1", []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}}, PermissionUser, "explicit response", "policy-v1", "explicit CLI batch decision", ""); err != nil {
			t.Fatal(err)
		}
		task, err := store.Read("batch-partial")
		if err != nil {
			t.Fatal(err)
		}
		if task.Status != TaskInterventionRequired || task.Permissions[0].Source != PermissionUser || task.Permissions[1].Disposition != PermissionPending {
			t.Fatalf("partial batch lost pending state or user provenance: %+v", task)
		}
	})
}

func TestPermissionBatchRejectsStaleAttempt(t *testing.T) {
	store, _ := createPendingPermissionBatch(t, "batch-stale-attempt")
	if err := store.Update("batch-stale-attempt", "attempt_started", "A newer attempt started", func(task *Task) error {
		task.Attempts = append(task.Attempts, Attempt{ID: "attempt-2", Agent: "reviewer", Status: "running"})
		task.Status = TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ResolvePermissionBatchWithPolicy("batch-stale-attempt", "attempt-1", []PermissionBatchDecision{{RequestID: "request-a", OptionID: "allow"}}, PermissionUser, "explicit response", "policy-v1", "explicit CLI batch decision", ""); err == nil {
		t.Fatal("accepted a stale-attempt permission")
	}
	task, err := store.Read("batch-stale-attempt")
	if err != nil || task.Permissions[0].Disposition != PermissionPending {
		t.Fatalf("stale-attempt batch mutated a pending record: %+v, %v", task, err)
	}
}

func TestPermissionBatchRejectsNonPendingRequestAtomically(t *testing.T) {
	store, _ := createPendingPermissionBatch(t, "batch-non-pending")
	if err := store.ResolvePermission("batch-non-pending", "attempt-1", "request-b", "allow", PermissionUser, "explicit response", "policy-v1", "explicit CLI decision"); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read("batch-non-pending")
	if err != nil {
		t.Fatal(err)
	}
	err = store.ResolvePermissionBatchWithPolicy("batch-non-pending", "attempt-1", []PermissionBatchDecision{
		{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-b", OptionID: "reject"},
	}, PermissionSourcePrincipal, "bounded task mandate", "policy-v1", "explicit CLI batch decision", "task mandate")
	if err == nil {
		t.Fatal("accepted a batch containing an already resolved request")
	}
	after, err := store.Read("batch-non-pending")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.Permissions[0].Disposition != PermissionPending || after.Permissions[1].Disposition != PermissionResolved {
		t.Fatalf("non-pending batch partially changed durable state: before=%+v after=%+v", before, after)
	}
}

func TestPermissionBatchRejectsCrossAttemptRequestAtomically(t *testing.T) {
	store, _ := createPendingPermissionBatch(t, "batch-cross-attempt")
	if err := store.Update("batch-cross-attempt", "permission_recorded", "Request belongs to an older attempt", func(task *Task) error {
		task.Permissions[1].AttemptID = "attempt-0"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read("batch-cross-attempt")
	if err != nil {
		t.Fatal(err)
	}
	err = store.ResolvePermissionBatchWithPolicy("batch-cross-attempt", "attempt-1", []PermissionBatchDecision{
		{RequestID: "request-a", OptionID: "allow"}, {RequestID: "request-b", OptionID: "allow"},
	}, PermissionSourcePrincipal, "bounded task mandate", "policy-v1", "explicit CLI batch decision", "task mandate")
	if err == nil {
		t.Fatal("accepted a batch containing a request from another attempt")
	}
	after, err := store.Read("batch-cross-attempt")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.Permissions[0].Disposition != PermissionPending || after.Permissions[1].Disposition != PermissionPending {
		t.Fatalf("cross-attempt batch partially changed durable state: before=%+v after=%+v", before, after)
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

func TestCancelRecoversWhenRecordedWorkerIdentityIsGone(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "lost-cancel", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("lost-cancel", "cancel_pending", "Cancellation was interrupted", func(task *Task) error {
		task.Status = TaskCancelling
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "running", ProcessID: 2147483000, ProcessStart: "0123456789abcdef0123456789abcdef"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Cancel(context.Background(), "lost-cancel"); err != nil {
		t.Fatal(err)
	}
	task, err := store.Read("lost-cancel")
	if err != nil || task.Status != TaskCancelled || task.Events[len(task.Events)-1].Type != "cancelled" {
		t.Fatalf("dead recorded worker did not reach cancelled: %+v, %v", task, err)
	}
}

func TestInboxGroupsIndependentPermissionsByTask(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "inbox-permissions", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("inbox-permissions", "started", "Worker started", func(task *Task) error {
		task.Status = TaskRunning
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "running"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"permission-a", "permission-b"} {
		if err := store.RecordPermission("inbox-permissions", PermissionRecord{RequestID: id, AttemptID: "attempt-1", Capability: "file-write", Options: []PermissionChoice{{OptionID: "allow", Name: "Allow"}}, Disposition: PermissionPending}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Create(Task{ID: "inbox-result", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("inbox-result", "started", "Worker started", func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("inbox-result", "result_ready", "Result is ready", func(task *Task) error { task.Status = TaskResultReady; return nil }); err != nil {
		t.Fatal(err)
	}
	items, err := store.Inbox()
	if err != nil || len(items) != 2 {
		t.Fatalf("inbox = %+v, %v", items, err)
	}
	byID := map[string]InboxItem{}
	for _, item := range items {
		byID[item.TaskID] = item
	}
	permission := byID["inbox-permissions"]
	if permission.Status != TaskInterventionRequired || len(permission.PendingRequestIDs) != 2 || len(permission.Kinds) != 2 || permission.Revision == 0 {
		t.Fatalf("permission requests were not grouped with task provenance: %+v", permission)
	}
	if byID["inbox-result"].Kinds[0] != "result" {
		t.Fatalf("result action missing from inbox: %+v", byID["inbox-result"])
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

func TestReconcileCapacityKeepsReservationWhenWorkerIdentityIsUnavailable(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "unknown-worker", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.writeCapacity(CapacityState{Reservations: []CapacityReservation{{
		TaskID: "unknown-worker", Keys: []string{"entry"},
		Worker: ProcessIdentity{PID: os.Getpid(), GroupID: os.Getpid(), StartToken: "invalid-token"},
	}}}); err != nil {
		t.Fatal(err)
	}
	released, err := store.ReconcileCapacity(time.Now())
	if err == nil || len(released) != 0 {
		t.Fatalf("unavailable worker inspection released capacity: %v, %v", released, err)
	}
	state, readErr := store.readCapacity()
	if readErr != nil || len(state.Reservations) != 1 || state.Reservations[0].TaskID != "unknown-worker" {
		t.Fatalf("unavailable worker inspection changed reservation: %+v, %v", state, readErr)
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
