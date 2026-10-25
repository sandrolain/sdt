package acpbridge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

// queued is how many questions a queue holds, read under its lock: the tests observe
// the queue from another goroutine, which the race detector is right to flag on a bare
// field read.
func queued(queue *ElicitationQueue) int {
	queue.q.mu.Lock()
	defer queue.q.mu.Unlock()
	return len(queue.q.queue)
}

// formRequest is the protocol's request shape, spelled once so the tests read like the
// wire rather than like the SDK's constructor defaults.
func formRequest(message string, properties map[string]any, required ...string) acp.UnstableCreateElicitationRequest {
	return acp.UnstableCreateElicitationRequest{Form: &acp.UnstableCreateElicitationForm{
		Mode:    "form",
		Message: message,
		RequestedSchema: acp.UnstableElicitationSchema{
			Type:        "object",
			Properties:  properties,
			Required:    required,
			Title:       acp.Ptr("Question"),
			Description: acp.Ptr(message),
		},
	}}
}

func stringProperty() map[string]any {
	return map[string]any{"branch": map[string]any{"type": "string", "title": "branch"}}
}

// newElicitationClient is a client over its own question queue, publishing cards into
// the channel the test reads.
func newElicitationClient(cards chan Elicitation, session string) *client {
	return &client{
		perms:        NewPermissionQueue(nil),
		elicitations: NewElicitationQueue(func(_ context.Context, request Elicitation) { cards <- request }),
		currentSession: func() string {
			return session
		},
	}
}

// An accepted question carries the reader's values back to the agent, and the published
// card names the conversation the question belongs to.
func TestAnElicitationIsAcceptedWithItsContent(t *testing.T) {
	cards := make(chan Elicitation, 4)
	c := newElicitationClient(cards, "sess-7")

	responses := make(chan acp.UnstableCreateElicitationResponse, 1)
	go func() {
		response, _ := c.UnstableCreateElicitation(context.Background(),
			formRequest("which branch?", stringProperty(), "branch"))
		responses <- response
	}()

	card := <-cards
	if card.SessionID != "sess-7" || card.Mode != ElicitationForm || card.Message != "which branch?" {
		t.Fatalf("the published question = %+v", card)
	}
	if card.RequestID == "" {
		t.Fatal("the question reached the browser with no id to answer")
	}
	if card.Schema["required"] == nil {
		t.Fatalf("the requested schema lost its required names: %+v", card.Schema)
	}
	if !c.elicitations.Answer(card.RequestID, ElicitationAnswer{
		Action:  ActionAccept,
		Content: map[string]any{"branch": "main"},
	}) {
		t.Fatal("the question was not waiting when it was answered")
	}

	response := <-responses
	if response.Accept == nil {
		t.Fatalf("the agent got %+v, want an accept", response)
	}
	if got := response.Accept.Content["branch"]; got != "main" {
		t.Fatalf("the accepted content = %v, want branch=main", response.Accept.Content)
	}
}

// A declined question tells the agent so and carries nothing.
func TestADeclinedElicitationCarriesNoContent(t *testing.T) {
	cards := make(chan Elicitation, 1)
	c := newElicitationClient(cards, "sess-1")

	responses := make(chan acp.UnstableCreateElicitationResponse, 1)
	go func() {
		response, _ := c.UnstableCreateElicitation(context.Background(),
			formRequest("anything?", stringProperty()))
		responses <- response
	}()
	card := <-cards
	c.elicitations.Answer(card.RequestID, ElicitationAnswer{Action: ActionDecline})

	response := <-responses
	if response.Decline == nil || response.Accept != nil {
		t.Fatalf("the agent got %+v, want a decline with no content", response)
	}
}

// A connection whose context ends answers the protocol's cancel, so an agent is never
// left waiting for a reader who is gone, and the queue is left empty.
func TestAnUnansweredElicitationIsCancelled(t *testing.T) {
	cards := make(chan Elicitation, 1)
	c := newElicitationClient(cards, "sess-1")

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	response, err := c.UnstableCreateElicitation(ctx, formRequest("still there?", stringProperty()))
	if err != nil {
		t.Fatalf("a cancelled question must still answer the agent: %v", err)
	}
	if response.Cancel == nil {
		t.Fatalf("the agent got %+v, want the protocol's cancellation", response)
	}
	if got := queued(c.elicitations); got != 0 {
		t.Fatalf("%d questions were left in the queue after the wait ended", got)
	}
}

// A second question waits its turn rather than replacing the first: the reader sees one
// card at a time, the same rule the permission queue keeps.
func TestASecondElicitationWaitsItsTurn(t *testing.T) {
	cards := make(chan Elicitation, 4)
	c := newElicitationClient(cards, "sess-1")

	for _, message := range []string{"first", "second"} {
		go func(message string) {
			_, _ = c.UnstableCreateElicitation(context.Background(),
				formRequest(message, stringProperty()))
		}(message)
	}

	first := <-cards
	select {
	case second := <-cards:
		t.Fatalf("a second card was published while %q was still waiting: %+v", first.Message, second)
	case <-time.After(50 * time.Millisecond):
	}
	c.elicitations.Answer(first.RequestID, ElicitationAnswer{Action: ActionDecline})

	select {
	case second := <-cards:
		// Whichever goroutine reached the queue first, the promoted question is the
		// other one: the test asserts the queue's order, not the scheduler's.
		if second.Message == first.Message || second.RequestID == first.RequestID {
			t.Fatalf("the promoted question = %+v, want the one that was waiting", second)
		}
		c.elicitations.Answer(second.RequestID, ElicitationAnswer{Action: ActionDecline})
	case <-time.After(time.Second):
		t.Fatal("the queued question never became current")
	}
}

// A session that ends releases the questions it holds, so a new chat cannot inherit a
// card nobody can answer.
func TestASessionEndingDropsItsQuestions(t *testing.T) {
	queue := NewElicitationQueue(nil)
	c := &client{
		perms:          NewPermissionQueue(nil),
		elicitations:   queue,
		currentSession: func() string { return "sess-9" },
	}
	done := make(chan acp.UnstableCreateElicitationResponse, 1)
	go func() {
		response, _ := c.UnstableCreateElicitation(context.Background(),
			formRequest("late?", stringProperty()))
		done <- response
	}()

	deadline := time.Now().Add(time.Second)
	for queued(queue) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if dropped := queue.CancelSession("sess-9"); dropped != 1 {
		t.Fatalf("CancelSession dropped %d questions, want 1", dropped)
	}
	select {
	case response := <-done:
		if response.Cancel == nil {
			t.Fatalf("a dropped question answered %+v, want cancel", response)
		}
	case <-time.After(time.Second):
		t.Fatal("a dropped question never answered the agent")
	}
}

// The thread keeps the question on the turn it belongs to, records the answer in place
// and marks a URL question completed without moving it.
func TestTheThreadFoldsTheQuestionAndItsAnswer(t *testing.T) {
	state := NewThreadState("sess-1")
	apply(t, state, userChunk("do the thing", ""))

	state.Elicitation(context.Background(), Elicitation{
		RequestID: "elicit-1", SessionID: "sess-1", Mode: ElicitationForm, Message: "which?",
	})
	turn := state.Snapshot().Messages
	parts := turn[len(turn)-1].Parts
	part := parts[len(parts)-1]
	if part.Kind != PartElicitation || part.Elicitation == nil || part.Elicitation.Message != "which?" {
		t.Fatalf("the question did not reach the turn: %+v", part)
	}

	if !state.ResolveElicitation("elicit-1", DecisionAccepted, map[string]any{"branch": "main"}) {
		t.Fatal("the answer did not resolve the question")
	}
	turn = state.Snapshot().Messages
	parts = turn[len(turn)-1].Parts
	resolved := parts[len(parts)-1]
	if resolved.Elicitation.Decision != DecisionAccepted || resolved.Elicitation.Content["branch"] != "main" {
		t.Fatalf("the resolved question = %+v", resolved.Elicitation)
	}

	state.Elicitation(context.Background(), Elicitation{
		RequestID: "elicit-2", SessionID: "sess-1", Mode: ElicitationURL,
		Message: "open it", URL: "https://example.com", ElicitationID: "e-2",
	})
	if !state.CompleteElicitation("e-2") {
		t.Fatal("the protocol's completion did not find the question")
	}
	turn = state.Snapshot().Messages
	parts = turn[len(turn)-1].Parts
	if last := parts[len(parts)-1]; !last.Elicitation.Completed {
		t.Fatal("the completed URL question is not marked completed")
	}
}

// The whole path, over a real connection: the agent asks while a turn is running, the
// bridge publishes the card, and the browser's answer reaches the agent as the protocol's
// own accept.
func TestTheAgentAsksOverTheWireAndTheReaderAnswers(t *testing.T) {
	answers := make(chan map[string]any, 1)
	agent := newFakeAgent(t, func(peer *fakeAgent, method string, _ json.RawMessage) any {
		switch method {
		case "session/prompt":
			go func() {
				answers <- peer.Ask(t, "elicitation/create", map[string]any{
					"mode":    "form",
					"message": "which branch?",
					"requestedSchema": map[string]any{
						"type":       "object",
						"properties": stringProperty(),
						"required":   []string{"branch"},
					},
				})
			}()
			return map[string]any{"stopReason": "end_turn"}
		}
		return agentVersion(acp.ProtocolVersionNumber, "sess-1")(peer, method, nil)
	})
	bridge := testBridge(t)
	sessionID, err := bridge.Activate(context.Background(), agent.Path())
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	session, ok := bridge.Session(sessionID)
	if !ok {
		t.Fatalf("the bridge holds no session %q", sessionID)
	}

	prompted := make(chan struct{})
	go func() {
		defer close(prompted)
		if err := session.Prompt(context.Background(), "go"); err != nil {
			t.Errorf("prompt: %v", err)
		}
	}()

	// The card is the thread's, not the test's: wait for it to arrive there.
	var requestID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, message := range session.State().Snapshot().Messages {
			for _, part := range message.Parts {
				if part.Kind == PartElicitation && part.Elicitation != nil {
					requestID = part.Elicitation.RequestID
				}
			}
		}
		if requestID != "" {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("the question never reached the thread")
	}

	if !bridge.Elicitations().Answer(requestID, ElicitationAnswer{
		Action:  ActionAccept,
		Content: map[string]any{"branch": "main"},
	}) {
		t.Fatal("the question was not waiting when the browser answered it")
	}

	select {
	case envelope := <-answers:
		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("the agent's reply = %v, want a result", envelope)
		}
		if result["action"] != ActionAccept {
			t.Fatalf("the agent's reply action = %v, want accept", result["action"])
		}
		content, _ := result["content"].(map[string]any)
		if content["branch"] != "main" {
			t.Fatalf("the agent's accepted content = %v", result["content"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the agent never received its answer")
	}
	<-prompted
}
