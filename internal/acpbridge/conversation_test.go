package acpbridge

import (
	"context"
	"errors"
	"testing"
)

// methodCount counts the times the agent was asked a method, which is how a test sees that
// a join did not open a session.
func methodCount(agent *fakeAgent, method string) int {
	count := 0
	for _, asked := range agent.Methods() {
		if asked == method {
			count++
		}
	}
	return count
}

// A conversation outlives the request that opened it: the panel closing its stream must not
// call off a turn the reader asked for, so the session's life is the bridge's and not the
// caller's. The turn runs under session.Context(), which is the context the viewer hands it.
func TestAConversationOutlivesTheContextThatOpenedIt(t *testing.T) {
	bridge, _ := activatedBridge(t, streamingAgent("still here"))

	ctx, cancel := context.WithCancel(context.Background())
	session, err := bridge.CurrentSession(ctx)
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	cancel() // the browser went away

	if err := session.Prompt(session.Context(), "una domanda"); err != nil {
		t.Fatalf("the turn died with the request that opened the conversation: %v", err)
	}
	if messages := session.State().Snapshot().Messages; len(messages) != 2 {
		t.Fatalf("the turn left %d messages, want the user's and the answer", len(messages))
	}
}

// A second stream joins the conversation the bridge holds instead of opening another: that
// is what makes a reopen free of session/new. Activation itself opens the one session.
func TestCurrentSessionJoinsTheOneTheBridgeHolds(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent("hello"))

	first, err := bridge.CurrentSession(context.Background())
	if err != nil {
		t.Fatalf("first CurrentSession: %v", err)
	}
	second, err := bridge.CurrentSession(context.Background())
	if err != nil {
		t.Fatalf("second CurrentSession: %v", err)
	}
	if second != first {
		t.Fatalf("a second browser got its own conversation (%q, then %q)", first.ID(), second.ID())
	}
	if got := methodCount(agent, "session/new"); got != 1 {
		t.Fatalf("the agent was asked session/new %d times, want the one activation opened", got)
	}
}

// A new chat is the reader's explicit act: it ends the conversation it replaces and makes
// the fresh one the conversation a browser joining the bridge is handed.
func TestNewConversationEndsTheOneItReplaced(t *testing.T) {
	bridge, _ := activatedBridge(t, streamingAgent("hello"))

	previous, err := bridge.CurrentSession(context.Background())
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if err := previous.Prompt(previous.Context(), "una domanda"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	id, err := bridge.NewConversation(context.Background())
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if id == previous.ID() {
		t.Fatal("a new chat must not hand back the conversation it replaced")
	}
	if _, still := bridge.Session(previous.ID()); still {
		t.Fatalf("the replaced conversation %q is still registered", previous.ID())
	}
	next, live := bridge.Session(id)
	if !live {
		t.Fatalf("the new conversation %q is not registered", id)
	}
	if got := bridge.SessionID(); got != id {
		t.Fatalf("the bridge hands out %q, want the new conversation %q", got, id)
	}
	if messages := next.State().Snapshot().Messages; len(messages) != 0 {
		t.Fatalf("a new chat starts with %d messages, want an empty thread", len(messages))
	}
	// The turn of the replaced conversation is called off with it: its life is not the new
	// conversation's.
	if err := previous.Context().Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("the replaced conversation's life is %v, want it ended", err)
	}
}

// An inert bridge has no conversation to join and none to replace, and it says so with the
// same refusal every command path uses.
func TestCurrentSessionAndNewConversationNeedAnActivatedBridge(t *testing.T) {
	bridge := New(Options{Root: t.TempDir()})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.CurrentSession(context.Background()); !errors.Is(err, ErrNotActivated) {
		t.Fatalf("CurrentSession on an inert bridge = %v, want ErrNotActivated", err)
	}
	if _, err := bridge.NewConversation(context.Background()); !errors.Is(err, ErrNotActivated) {
		t.Fatalf("NewConversation on an inert bridge = %v, want ErrNotActivated", err)
	}
}
