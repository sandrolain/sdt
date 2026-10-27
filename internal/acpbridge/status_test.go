package acpbridge

import "testing"

// TestStatusListenerSeesEachChange is the O1 gate on the bridge's side: every
// posture change reaches the listener once, with the sentence beside it, and a
// repeated state is not reported again.
func TestStatusListenerSeesEachChange(t *testing.T) {
	b := New(Options{})
	var got []Posture
	b.ListenStatus(func(p Posture) { got = append(got, p) })

	b.setStatus(StatusStarting, "")
	b.setStatus(StatusStarting, "") // the same posture: not a change
	b.setStatus(StatusActive, "")
	b.setNote("could not put the agent back on its conversation")

	if len(got) != 3 {
		t.Fatalf("listener saw %d changes, want 3: %#v", len(got), got)
	}
	if got[0].Status != StatusStarting || got[1].Status != StatusActive {
		t.Fatalf("states out of order: %#v", got)
	}
	if got[2].Status != StatusActive || got[2].Error != "could not put the agent back on its conversation" {
		t.Fatalf("the sentence did not travel with the posture: %#v", got[2])
	}
}

// TestPostureReportsTheSessionAndModel pins the two fields the frame carries
// beyond the state itself.
func TestPostureReportsTheSessionAndModel(t *testing.T) {
	b := New(Options{Models: []string{"opencode/example"}})
	b.mu.Lock()
	b.initial = "sess-7"
	b.mu.Unlock()

	p := b.Posture()
	if p.SessionID != "sess-7" || p.Model != "opencode/example" || p.Status != StatusIdle {
		t.Fatalf("posture = %#v", p)
	}
}
