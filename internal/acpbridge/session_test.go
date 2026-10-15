package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

// streamingAgent answers a session/prompt by streaming one chunk to the client and
// then finishing the turn: the shape of a real agent, driven over the wire.
func streamingAgent(t *testing.T, text string) handler {
	t.Helper()
	return func(agent *fakeAgent, method string, _ json.RawMessage) any {
		switch method {
		case "initialize":
			return map[string]any{
				"protocolVersion":   acp.ProtocolVersionNumber,
				"agentCapabilities": map[string]any{},
				"authMethods":       []any{},
			}
		case "session/new":
			return map[string]any{"sessionId": "sess-1"}
		case "session/prompt":
			agent.notify(t, "session/update", map[string]any{
				"sessionId": "sess-1",
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]any{"type": "text", "text": text},
				},
			})
			return map[string]any{"stopReason": "end_turn"}
		case "session/cancel":
			return nil
		}
		return rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

// waitForMethod waits until the agent has been asked a method: the client writes
// through its own goroutine, so a test waits for the observable rather than
// assuming the write already landed.
func waitForMethod(t *testing.T, agent *fakeAgent, method string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, asked := range agent.methods() {
			if asked == method {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the agent was never asked %s (asked: %v)", method, agent.methods())
}

func activatedBridge(t *testing.T, h handler) (*Bridge, *fakeAgent) {
	t.Helper()
	agent := newFakeAgent(t, h)
	bridge := New(Options{Root: t.TempDir(), Version: "test"})
	if _, err := bridge.Activate(context.Background(), agent.path); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	return bridge, agent
}

// A turn over the wire: the prompt carries the user's text, the agent's chunks fold
// into the thread, and the user's own words stay visible even though this agent
// never echoes them.
func TestPromptStreamsTheTurnIntoTheThread(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent(t, "The corpus has 224 analyses."))
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}

	states := make(chan Thread, 8)
	session.State().Listen(func(thread Thread) { states <- thread })

	if err := session.Prompt(context.Background(), "how big is the corpus?"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	prompted := agent.paramsFor(t, "session/prompt")
	prompt, ok := prompted["prompt"].([]any)
	if !ok || len(prompt) != 1 {
		t.Fatalf("the prompt carried %v", prompted["prompt"])
	}
	block, ok := prompt[0].(map[string]any)
	if !ok || block["text"] != "how big is the corpus?" {
		t.Fatalf("the prompt text = %v", prompt[0])
	}
	if got := prompted["sessionId"]; got != "sess-1" {
		t.Fatalf("the prompt named session %v", got)
	}

	// The listener saw the prompt go out and the agent's chunk arrive.
	var last Thread
	for {
		select {
		case last = <-states:
			continue
		case <-time.After(time.Second):
		}
		break
	}
	thread := session.State().Snapshot()
	if len(thread.Messages) != 2 {
		t.Fatalf("the thread holds %d messages: %+v", len(thread.Messages), thread.Messages)
	}
	if thread.Messages[0].Role != RoleUser || thread.Messages[0].Parts[0].Text != "how big is the corpus?" {
		t.Fatalf("the user's message = %+v", thread.Messages[0])
	}
	if thread.Messages[1].Role != RoleAssistant || thread.Messages[1].Parts[0].Text != "The corpus has 224 analyses." {
		t.Fatalf("the agent's message = %+v", thread.Messages[1])
	}
	if len(last.Messages) != 2 {
		t.Fatalf("the last streamed state held %d messages", len(last.Messages))
	}
}

// Cancel is a notification: it reaches the agent and the turn ends where it is.
func TestCancelReachesTheAgent(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent(t, "almost done"))
	session, _ := bridge.Session(bridge.SessionID())
	if err := session.Prompt(context.Background(), "go"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if err := session.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForMethod(t, agent, "session/cancel")
}

// A browser that goes away releases the agent: every wait of its session ends at
// once, and the one after it becomes current.
func TestCancellingASessionReleasesItsWaits(t *testing.T) {
	cards := make(chan Permission, 4)
	queue := NewPermissionQueue(func(_ context.Context, request Permission) { cards <- request })

	released := make(chan error, 1)
	go func() {
		_, err := queue.Ask(context.Background(), Permission{SessionID: "sess-1", Title: "first"})
		released <- err
	}()
	if card := <-cards; card.SessionID != "sess-1" {
		t.Fatalf("the first card belongs to %q", card.SessionID)
	}

	waiting := make(chan error, 1)
	go func() {
		_, err := queue.Ask(context.Background(), Permission{SessionID: "sess-2", Title: "second"})
		waiting <- err
	}()

	if got := queue.CancelSession("sess-1"); got != 1 {
		t.Fatalf("CancelSession cancelled %d waits, want 1", got)
	}
	if err := <-released; !errors.Is(err, ErrCancelled) {
		t.Fatalf("the cancelled wait returned %v, want ErrCancelled", err)
	}

	// The session that is still there becomes current and is answered normally.
	var promoted Permission
	select {
	case promoted = <-cards:
	case <-time.After(time.Second):
		t.Fatal("the remaining wait was never promoted")
	}
	if promoted.SessionID != "sess-2" {
		t.Fatalf("the promoted card belongs to %q", promoted.SessionID)
	}
	if !queue.Answer(promoted.RequestID, true) {
		t.Fatal("the promoted card was not waiting")
	}
	if err := <-waiting; err != nil {
		t.Fatalf("the answered wait returned %v", err)
	}
}

// Closing one session stops its conversation; the connection stays for the next
// browser.
func TestClosingASessionLeavesTheConnectionUp(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent(t, "hello"))
	session, _ := bridge.Session(bridge.SessionID())
	if cancelled := session.Close(); cancelled != 0 {
		t.Fatalf("closing an idle session cancelled %d waits", cancelled)
	}
	if _, ok := bridge.Session(session.ID()); ok {
		t.Fatal("the closed session is still in the registry")
	}
	if !bridge.Activated() {
		t.Fatal("closing a session closed the connection")
	}
	if got := len(agent.methods()); got == 0 {
		t.Fatal("the agent saw nothing")
	}
}
