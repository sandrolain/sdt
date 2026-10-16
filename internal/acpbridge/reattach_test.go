package acpbridge

import (
	"context"
	"strings"
	"testing"
	"time"
)

// restartWith shrinks the restart delay so a test can exercise a death and the attempt
// that follows it without waiting on the shipped bound.
func restartWith(t *testing.T) {
	t.Helper()
	old := restartDelay
	restartDelay = 20 * time.Millisecond
	t.Cleanup(func() { restartDelay = old })
}

// TestARestartTakesTheSessionBack covers the promise of the reattach: an agent that died
// is asked to take its session back, the conversation the reader was looking at is still
// there afterwards, and the replay the agent sends is not folded onto it — the thread is
// the display source, and folding the replay would show every message twice.
func TestARestartTakesTheSessionBack(t *testing.T) {
	restartWith(t)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	session, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperLoadMode), root)
	if err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	conversation, live := bridge.Session(session)
	if !live {
		t.Fatalf("the session %q must be in the registry", session)
	}
	if err := conversation.Prompt(context.Background(), "una domanda"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	before := len(conversation.State().Snapshot().Messages)
	if before == 0 {
		t.Fatal("the turn must have left the thread with something to keep")
	}
	beforeText := threadText(conversation.State().Snapshot())

	if err := syscallKill(bridge.agentPID()); err != nil {
		t.Fatalf("killing the agent: %v", err)
	}
	waitFor(t, "the agent to be started again (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return len(startedPIDs(t, scratch)) >= 2 && status == StatusActive
	})

	if !bridge.SupportsSessionLoad() {
		t.Fatal("the agent advertised the capability, so the bridge must have read it")
	}
	if !wasAsked(t, scratch, "session/load") {
		t.Fatal("the restart must ask the agent to take the session back")
	}
	if got := bridge.SessionID(); got != session {
		t.Fatalf("the conversation keeps its session: want %q, got %q", session, got)
	}
	// The probe is the replay's own text: a folded replay reconciles with the messages
	// already there rather than adding new ones, so a count would not catch it — the
	// words would show up twice.
	after := conversation.State().Snapshot()
	if threadText(after) != beforeText {
		t.Fatalf("the replay must not be folded onto the thread we already had:\n before: %q\n after:  %q", beforeText, threadText(after))
	}
	if len(after.Messages) != before {
		t.Fatalf("the replay must not add messages: %d before, %d after", before, len(after.Messages))
	}
	if status, note := bridge.Status(); status != StatusActive || note != "" {
		t.Fatalf("a successful reattach carries no note: status=%q note=%q", status, note)
	}
	// The same conversation object is still the browser's, and still works: the turn
	// goes to the new process over the changed connection.
	if err := conversation.Prompt(context.Background(), "e ancora"); err != nil {
		t.Fatalf("the conversation must keep working after a restart: %v", err)
	}
}

// TestARestartOnAnEmptyThreadAcceptsTheReplay covers the other half of the rule: a thread
// that holds nothing is the one case where the replay is the only account of the
// conversation, so it is kept. It is what a viewer that restarted finds.
func TestARestartOnAnEmptyThreadAcceptsTheReplay(t *testing.T) {
	restartWith(t)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	session, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperLoadMode), root)
	if err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	conversation, _ := bridge.Session(session)

	if err := syscallKill(bridge.agentPID()); err != nil {
		t.Fatalf("killing the agent: %v", err)
	}
	waitFor(t, "the replay to fill the empty thread", func() bool {
		status, _ := bridge.Status()
		return status == StatusActive && len(conversation.State().Snapshot().Messages) > 0
	})
	if asked := askedMethods(t, scratch); len(asked) == 0 {
		t.Fatal("the child must have been asked something")
	}
}

// TestARestartWithoutTheCapabilityDegradesToAFreshSession covers the degradation both
// ways: an agent that never advertised the capability is not asked to load, and one that
// advertised it and refused is asked and refused. Either way the thread the reader was
// looking at is kept, and the bridge says the agent's own context was let go of.
func TestARestartWithoutTheCapabilityDegradesToAFreshSession(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		asked    bool
		restarts int
	}{
		{name: "an agent that cannot load", mode: "", asked: false},
		{name: "an agent that advertises it and refuses", mode: spawnHelperRefuseMode, asked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restartWith(t)
			scratch, root := t.TempDir(), t.TempDir()
			bridge := New(Options{Root: root})
			t.Cleanup(func() { _ = bridge.Close() })

			session, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, tc.mode), root)
			if err != nil {
				t.Fatalf("EnsureStarted: %v", err)
			}
			conversation, _ := bridge.Session(session)
			if err := conversation.Prompt(context.Background(), "una domanda"); err != nil {
				t.Fatalf("Prompt: %v", err)
			}
			before := len(conversation.State().Snapshot().Messages)

			if err := syscallKill(bridge.agentPID()); err != nil {
				t.Fatalf("killing the agent: %v", err)
			}
			waitFor(t, "the bridge to be active again (status="+bridge.statusLine()+")", func() bool {
				status, _ := bridge.Status()
				return len(startedPIDs(t, scratch)) >= 2 && status == StatusActive
			})

			if got := wasAsked(t, scratch, "session/load"); got != tc.asked {
				t.Fatalf("session/load asked: want %v, got %v", tc.asked, got)
			}
			status, note := bridge.Status()
			if status != StatusActive {
				t.Fatalf("a degraded reattach leaves the bridge working: status=%q", status)
			}
			if !strings.Contains(note, "context") {
				t.Fatalf("the reader must be told the agent's context was let go of, got %q", note)
			}
			if after := len(conversation.State().Snapshot().Messages); after != before {
				t.Fatalf("the conversation must be kept across the degradation: %d then %d", before, after)
			}
			// A fresh session is what the agent was asked for, and the conversation
			// continues on it.
			if !wasAsked(t, scratch, "session/new") {
				t.Fatal("a degradation must open a fresh session")
			}
			if err := conversation.Prompt(context.Background(), "e ancora"); err != nil {
				t.Fatalf("the conversation must keep working on the fresh session: %v", err)
			}
		})
	}
}

// threadText renders every part of a thread, which is what a reader sees. A test
// compares it across a restart because a folded replay reconciles with the messages
// already there rather than adding new ones: the words are what shows it, not a count.
func threadText(thread Thread) string {
	var out strings.Builder
	for _, message := range thread.Messages {
		for _, part := range message.Parts {
			out.WriteString(part.Text)
			out.WriteString("\n")
		}
	}
	return out.String()
}
