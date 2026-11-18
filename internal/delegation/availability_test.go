package delegation

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

func TestClassifyACPFailureUsesOnlyStructuredProviderSignals(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		kind  FailureKind
		scope string
	}{
		{"auth code", &acp.RequestError{Code: -32000, Message: "auth required"}, FailureAuth, "entry"},
		{"quota marker", &acp.RequestError{Code: -32603, Data: map[string]any{"type": "quota_exceeded", "availability_scope": "group"}}, FailureQuota, "group"},
		{"unavailable marker", &acp.RequestError{Code: -32603, Data: map[string]any{"code": "provider_unavailable"}}, FailureUnavailable, "entry"},
		{"permission marker", &acp.RequestError{Code: -32603, Data: map[string]any{"type": "permission_denied"}}, FailurePermission, "entry"},
		{"message is not a signal", &acp.RequestError{Code: -32603, Message: "quota exhausted, try again"}, FailureUnknown, "entry"},
		{"generic error", errors.New("connection reset"), FailureUnknown, "entry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifyACPFailure(test.err)
			if got.Kind != test.kind || got.Scope != test.scope {
				t.Fatalf("ClassifyACPFailure() = %+v, want %s/%s", got, test.kind, test.scope)
			}
		})
	}
}

func TestAvailabilityExpiryUsesReportedTimeAndLocalBoundaries(t *testing.T) {
	zone := time.FixedZone("UTC+10", 10*60*60)
	now := time.Date(2026, 11, 16, 23, 30, 0, 0, zone)
	want := time.Date(2026, 11, 17, 0, 0, 0, 0, zone)
	if got := NextLocalMidnight(now, zone); !got.Equal(want) {
		t.Fatalf("next local midnight = %s, want %s", got, want)
	}
	reported := now.Add(17 * time.Minute)
	exclusion, err := availabilityExpiry("agent", "acct", FailureSignal{Kind: FailureQuota, Scope: "group", Until: reported}, 0, now)
	if err != nil || exclusion.Key != "group:acct" || !exclusion.Until.Equal(reported.UTC()) {
		t.Fatalf("reported expiry was not preserved: %+v, %v", exclusion, err)
	}
	if _, err := availabilityExpiry("agent", "", FailureSignal{Kind: FailureUnavailable}, 0, now); err == nil {
		t.Fatal("unavailability without a provider expiry or configured cooldown was accepted")
	}
}

func TestAvailabilityInspectionFiltersExpiryAndRecoveryProbeOwnership(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if _, err := store.RecordAvailabilityFailure("agent-a", "", FailureSignal{Kind: FailureUnavailable, Until: now.Add(time.Minute)}, time.Minute, now); err != nil {
		t.Fatal(err)
	}
	active, err := store.Availability(now)
	if err != nil || len(active) != 1 || active[0].Key != "entry:agent-a" {
		t.Fatalf("active availability = %+v, %v", active, err)
	}
	active, err = store.Availability(now.Add(time.Minute))
	if err != nil || len(active) != 0 {
		t.Fatalf("expired availability = %+v, %v", active, err)
	}
	claimed, _, err := store.ClaimCandidate("agent-a", "", "probe-owner", now.Add(2*time.Minute))
	if err != nil || !claimed {
		t.Fatalf("recovery probe claim = %v, %v", claimed, err)
	}
	owned, err := store.RecoveryProbes("probe-owner")
	if err != nil || len(owned) != 1 || owned[0].TaskID != "probe-owner" {
		t.Fatalf("owned recovery probes = %+v, %v", owned, err)
	}
	other, err := store.RecoveryProbes("other-task")
	if err != nil || len(other) != 0 {
		t.Fatalf("unowned recovery probes = %+v, %v", other, err)
	}
}

func TestFailureSignalValidation(t *testing.T) {
	for _, kind := range []FailureKind{FailureQuota, FailureUnavailable, FailureAuth, FailurePermission, FailureUnknown, FailureSuccess} {
		if err := (FailureSignal{Kind: kind}).Validate(); err != nil {
			t.Errorf("valid failure class %q: %v", kind, err)
		}
	}
	if err := (FailureSignal{Kind: FailureKind("unexpected")}).Validate(); err == nil {
		t.Fatal("unknown failure class was accepted")
	}
}

func TestAvailabilityScopesAndSingleRecoveryProbe(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)
	if _, err := store.RecordAvailabilityFailure("agent-a", "shared-account", FailureSignal{Kind: FailureQuota, Scope: "group", Until: until}, 0, now); err != nil {
		t.Fatal(err)
	}
	if available, err := store.CandidateAvailable("agent-b", "shared-account", "task-b", now); err != nil || available {
		t.Fatalf("group-scoped exclusion allowed another account member: %v, %v", available, err)
	}
	if available, err := store.CandidateAvailable("agent-b", "other-account", "task-b", now); err != nil || !available {
		t.Fatalf("scoped exclusion leaked to another group: %v, %v", available, err)
	}
	probeAt := until.Add(time.Second)
	var wg sync.WaitGroup
	var mu sync.Mutex
	claims := 0
	for _, taskID := range []string{"probe-a", "probe-b"} {
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			claimed, err := store.ClaimRecoveryProbe("group:shared-account", taskID, probeAt)
			if err != nil {
				t.Errorf("ClaimRecoveryProbe: %v", err)
			}
			if claimed {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}(taskID)
	}
	wg.Wait()
	if claims != 1 {
		t.Fatalf("simultaneous expiry admitted %d recovery probes, want exactly one", claims)
	}
	state, err := store.readAvailability()
	if err != nil {
		t.Fatal(err)
	}
	probeTask := state.Probes[0].TaskID
	if available, err := store.CandidateAvailable("agent-b", "shared-account", "other-task", probeAt); err != nil || available {
		t.Fatalf("other task bypassed the active recovery probe: %v, %v", available, err)
	}
	if available, err := store.CandidateAvailable("agent-b", "shared-account", probeTask, probeAt); err != nil || !available {
		t.Fatalf("probe owner was blocked: %v, %v", available, err)
	}
	if err := store.ResetAvailability("group:shared-account"); err == nil {
		t.Fatal("manual reset discarded an active probe")
	}
	if err := store.FinishRecoveryProbe("group:shared-account", probeTask, "agent-a", "shared-account", FailureSignal{Kind: FailureUnknown}, 0, probeAt); err != nil {
		t.Fatal(err)
	}
	if available, err := store.CandidateAvailable("agent-b", "shared-account", "next", probeAt); err != nil || !available {
		t.Fatalf("conclusive non-availability outcome retained an expired exclusion: %v, %v", available, err)
	}
}

func TestCrashedRecoveryProbeReappliesBoundedCooldown(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if _, err := store.RecordAvailabilityFailure("agent-a", "", FailureSignal{Kind: FailureUnavailable, Until: now.Add(time.Minute)}, time.Minute, now); err != nil {
		t.Fatal(err)
	}
	probeAt := now.Add(2 * time.Minute)
	claimed, err := store.ClaimRecoveryProbe("entry:agent-a", "probe-task", probeAt)
	if err != nil || !claimed {
		t.Fatalf("recovery probe = %v, %v", claimed, err)
	}
	if err := store.ReconcileRecoveryProbe("entry:agent-a", "probe-task", "agent-a", "", 3*time.Minute, probeAt); err != nil {
		t.Fatal(err)
	}
	available, err := store.CandidateAvailable("agent-a", "", "other-task", probeAt.Add(time.Minute))
	if err != nil || available {
		t.Fatalf("crashed probe left candidate available inside its cooldown: %v, %v", available, err)
	}
	if err := store.ResetAvailability("entry:agent-a"); err != nil {
		t.Fatal(err)
	}
}

func TestClaimCandidateLocksEntryAndGroupProbeTogether(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Minute)
	for _, failure := range []struct {
		agent string
		group string
		scope string
	}{{"agent-a", "shared", "entry"}, {"agent-a", "shared", "group"}} {
		if _, err := store.RecordAvailabilityFailure(failure.agent, failure.group, FailureSignal{Kind: FailureQuota, Scope: failure.scope, Until: until}, 0, now); err != nil {
			t.Fatal(err)
		}
	}
	probeAt := until.Add(time.Second)
	claimed, keys, err := store.ClaimCandidate("agent-a", "shared", "probe-a", probeAt)
	if err != nil || !claimed || len(keys) != 2 {
		t.Fatalf("candidate claim = %v, %v, %v; want atomic entry and group probes", claimed, keys, err)
	}
	claimed, _, err = store.ClaimCandidate("agent-b", "shared", "probe-b", probeAt)
	if err != nil || claimed {
		t.Fatalf("second group member claimed during the probe: %v, %v", claimed, err)
	}
	if err := store.FinishCandidateProbes(keys, "probe-a", "agent-a", "shared", FailureSignal{Kind: FailureQuota, Scope: "group"}, 0, probeAt); err != nil {
		t.Fatal(err)
	}
	if available, err := store.CandidateAvailable("agent-b", "shared", "probe-b", probeAt); err != nil || available {
		t.Fatalf("group quota outcome did not renew shared exclusion: %v, %v", available, err)
	}
}

func TestIndeterminateCandidateProbeGetsBoundedCooldown(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	if _, err := store.RecordAvailabilityFailure("agent-a", "", FailureSignal{Kind: FailureUnavailable, Until: now.Add(time.Minute)}, time.Minute, now); err != nil {
		t.Fatal(err)
	}
	probeAt := now.Add(2 * time.Minute)
	claimed, keys, err := store.ClaimCandidate("agent-a", "", "probe-task", probeAt)
	if err != nil || !claimed || len(keys) != 1 {
		t.Fatalf("probe claim = %v, %v, %v", claimed, keys, err)
	}
	if err := store.FinishCandidateProbes(keys, "probe-task", "agent-a", "", FailureSignal{Kind: FailureUnknown}, 3*time.Minute, probeAt); err != nil {
		t.Fatal(err)
	}
	available, err := store.CandidateAvailable("agent-a", "", "next-task", probeAt.Add(time.Minute))
	if err != nil || available {
		t.Fatalf("indeterminate probe bypassed its bounded cooldown: %v, %v", available, err)
	}
}

func TestCapacityWaitReleasesUnstartedRecoveryProbe(t *testing.T) {
	store := NewStore(shortProjectTemp(t))
	for _, id := range []string{"capacity-owner", "probe-waiter", "next-probe"} {
		if err := store.Create(Task{ID: id, Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if admitted, err := store.TryAdmit("capacity-owner", map[string]int{"agent-a": 1}); err != nil || !admitted {
		t.Fatalf("occupy capacity: %v, %v", admitted, err)
	}
	if err := store.Update("capacity-owner", "start", "hold capacity", func(task *Task) error { task.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordAvailabilityFailure("agent-a", "", FailureSignal{Kind: FailureUnavailable, Until: now.Add(-time.Second)}, time.Minute, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	claimed, keys, err := store.ClaimCandidate("agent-a", "", "probe-waiter", now)
	if err != nil || !claimed {
		t.Fatalf("claim expired candidate: %v, %v", claimed, err)
	}
	if admitted, err := store.TryAdmit("probe-waiter", map[string]int{"agent-a": 1}); err != nil || admitted {
		t.Fatalf("capacity should be full: %v, %v", admitted, err)
	}
	if err := store.ReleaseCandidateProbes(keys, "probe-waiter"); err != nil {
		t.Fatal(err)
	}
	claimed, _, err = store.ClaimCandidate("agent-a", "", "next-probe", now)
	if err != nil || !claimed {
		t.Fatalf("unstarted probe slot remained stranded: %v, %v", claimed, err)
	}
}
