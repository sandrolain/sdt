package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

func waitForMethod(t *testing.T, agent *fakeAgent, method string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, asked := range agent.Methods() {
			if asked == method {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the agent was never asked %s (asked: %v)", method, agent.Methods())
}

func activatedBridge(t *testing.T, h handler) (*Bridge, *fakeAgent) {
	t.Helper()
	agent := newFakeAgent(t, h)
	bridge := New(Options{Root: t.TempDir(), Version: "test"})
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	return bridge, agent
}

// A turn over the wire: the prompt carries the user's text, the agent's chunks fold
// into the thread, and the user's own words stay visible even though this agent
// never echoes them.
func TestPromptStreamsTheTurnIntoTheThread(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent("The corpus has 224 analyses."))
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}

	states := make(chan Thread, 8)
	session.State().Listen(func(thread Thread) { states <- thread })

	if err := session.Prompt(context.Background(), "how big is the corpus?"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	prompted := agent.ParamsFor(t, "session/prompt")
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
	bridge, agent := activatedBridge(t, streamingAgent("almost done"))
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
	bridge, agent := activatedBridge(t, streamingAgent("hello"))
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
	if got := len(agent.Methods()); got == 0 {
		t.Fatal("the agent saw nothing")
	}
}

// A second prompt while a turn runs is refused: two turns on one session would
// interleave updates the thread cannot attribute.
func TestASecondPromptIsRefusedWhileATurnRuns(t *testing.T) {
	release := make(chan struct{})
	handler := func(agent *fakeAgent, method string, params json.RawMessage) any {
		if method == "session/prompt" {
			<-release
			return map[string]any{"stopReason": "end_turn"}
		}
		return agentVersion(1, "sess-1")(agent, method, params)
	}
	bridge, agent := activatedBridge(t, handler)
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatal("the bridge holds no session")
	}

	first := make(chan error, 1)
	go func() { first <- session.Prompt(context.Background(), "first") }()
	waitForMethod(t, agent, "session/prompt")

	if err := session.Prompt(context.Background(), "second"); !errors.Is(err, ErrTurnRunning) {
		t.Fatalf("a second prompt returned %v, want ErrTurnRunning", err)
	}

	close(release)
	if err := <-first; err != nil {
		t.Fatalf("the first turn failed: %v", err)
	}
	// And the session takes the next turn once the first is done.
	if err := session.Prompt(context.Background(), "third"); err != nil {
		t.Fatalf("the turn after the first: %v", err)
	}
}

// A refused turn is not silence: the agent's own words land on the message that was sent,
// where the reader looks for the answer that did not come — and the caller still gets the
// error the viewer logs.
func TestARefusedTurnReachesTheThread(t *testing.T) {
	bridge, _ := activatedBridge(t, refusingAgent(-32603, "This request requires more credits"))
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatal("the bridge holds no session")
	}

	if err := session.Prompt(context.Background(), "how big is the corpus?"); err == nil {
		t.Fatal("a refused turn returned no error")
	}

	thread := session.State().Snapshot()
	if len(thread.Messages) != 1 {
		t.Fatalf("the thread holds %d messages: %+v", len(thread.Messages), thread.Messages)
	}
	message := thread.Messages[0]
	if message.Role != RoleUser || message.Provisional {
		t.Fatalf("the message carrying the refusal = %+v", message)
	}
	if n := len(message.Parts); n != 2 || message.Parts[n-1].Kind != PartError {
		t.Fatalf("the parts of the refused message = %+v", message.Parts)
	}
	if got := message.Parts[1].Text; got != "This request requires more credits" {
		t.Errorf("the refusal the reader is told = %q", got)
	}
}

// The reader is given the agent's sentence without the JSON-RPC envelope it travels in, and
// the error's own text when the error is not that shape at all.
func TestTurnFailureUsesTheAgentsOwnWords(t *testing.T) {
	refusal := fmt.Errorf("acpbridge: prompt: %w", &acp.RequestError{Code: -32603, Message: "the provider refused it"})
	if got := turnFailure(refusal); got != "the provider refused it" {
		t.Errorf("turnFailure of a refusal = %q", got)
	}
	plain := fmt.Errorf("acpbridge: prompt: %w", errors.New("the transport went away"))
	if got := turnFailure(plain); got != "acpbridge: prompt: the transport went away" {
		t.Errorf("turnFailure of a plain error = %q", got)
	}
	// A refusal with no sentence is the one case where the code is the report: the reader is
	// not handed a raw JSON-RPC envelope.
	if got := turnFailure(fmt.Errorf("acpbridge: prompt: %w", &acp.RequestError{Code: -32603})); got != "the agent refused the turn (code -32603)" {
		t.Errorf("a refusal with no sentence reported %q", got)
	}
}

// A turn that stopped for a reason says so; a turn that simply ended, or that the reader
// cancelled themselves, says nothing.
func TestOnlyTheExplanatoryStopsAreRecorded(t *testing.T) {
	explanatory := []acp.StopReason{acp.StopReasonMaxTokens, acp.StopReasonRefusal, acp.StopReasonMaxTurnRequests}
	for _, reason := range explanatory {
		bridge, _ := activatedBridge(t, stoppingAgent("a partial answer", string(reason)))
		session, _ := bridge.Session(bridge.SessionID())
		if err := session.Prompt(context.Background(), "how big is the corpus?"); err != nil {
			t.Fatalf("Prompt: %v", err)
		}
		parts := session.State().Snapshot().Messages[0].Parts
		if n := len(parts); n < 2 || parts[n-1].Kind != PartStop || parts[n-1].Text != string(reason) {
			t.Errorf("the recorded ending of a %s turn = %+v", reason, parts)
		}
	}

	silent := []acp.StopReason{acp.StopReasonEndTurn, acp.StopReasonCancelled}
	for _, reason := range silent {
		bridge, _ := activatedBridge(t, stoppingAgent("the answer", string(reason)))
		session, _ := bridge.Session(bridge.SessionID())
		if err := session.Prompt(context.Background(), "how big is the corpus?"); err != nil {
			t.Fatalf("Prompt: %v", err)
		}
		// This agent never echoes the user's own message (see acptest.Streams), so the flag can
		// only be cleared by the turn ending — and a turn that simply finished must not leave
		// the reader waiting on an answer that has already arrived.
		message := session.State().Snapshot().Messages[0]
		if message.Provisional {
			t.Errorf("a %s turn left %q waiting", reason, message.ID)
		}
		for _, part := range message.Parts {
			if part.Kind == PartStop || part.Kind == PartError {
				t.Errorf("a %s turn recorded %+v", reason, part)
			}
		}
	}
}

// modelInUse reads the model a thread says is in use. It is the field the panel builds its
// control from, so a test about what the reader sees reads this rather than the wire.
func modelInUse(t *testing.T, thread Thread) string {
	t.Helper()
	for _, raw := range thread.Config {
		var option struct {
			ID           string `json:"id"`
			CurrentValue string `json:"currentValue"`
		}
		if err := json.Unmarshal(raw, &option); err != nil {
			t.Fatalf("a config option in the thread is not the agent's own payload: %v (%s)", err, raw)
		}
		if option.ID == "model" {
			return option.CurrentValue
		}
	}
	t.Fatalf("the thread carries no model option: %v", thread.Config)
	return ""
}

// A set the agent accepts moves the session's own configuration: the answer carries the option
// with the value now in use, and it lands in the thread the panel reads. The control therefore
// shows the agent's word rather than the choice that was asked for — an agent is free to answer
// with something else, and the panel must not have guessed.
func TestASetTheAgentAcceptsMovesTheSessionsConfig(t *testing.T) {
	bridge, agent := activatedBridge(t, offeringAgent("openrouter/meta/muse-spark-open",
		"openrouter/meta/muse-spark-open", "opencode/big-pickle"))
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	states := make(chan Thread, 4)
	session.State().Listen(func(thread Thread) { states <- thread })

	if err := session.SetConfig(context.Background(), "model", "opencode/big-pickle"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	if got := modelInUse(t, session.State().Snapshot()); got != "opencode/big-pickle" {
		t.Fatalf("the thread is on %q, want the model the agent answered with", got)
	}
	select {
	case thread := <-states:
		if got := modelInUse(t, thread); got != "opencode/big-pickle" {
			t.Fatalf("the announced thread is on %q, want the agent's own answer", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the set changed the thread and nothing was announced to the browser")
	}
	asked := agent.ParamsFor(t, "session/set_config_option")
	if asked["configId"] != "model" || asked["value"] != "opencode/big-pickle" {
		t.Fatalf("the agent was asked %v, want the session's model set to the chosen value", asked)
	}
	if asked["sessionId"] != bridge.SessionID() {
		t.Fatalf("the set named session %v, want the conversation it was made on (%q)", asked["sessionId"], bridge.SessionID())
	}
}

// A declared model the agent will not take is a sentence, not a failed start: the session opens
// on the model the agent chose, the reader reads the agent's own words, and a turn still works.
// A refusal is a provider's answer about one model, not a reason to leave the reader with a dead
// panel.
func TestADeclaredModelTheAgentRefusesOpensTheSessionAnyway(t *testing.T) {
	agent := newFakeAgent(t, setRefusingAgent("the provider has no such model"))
	bridge := New(Options{Root: t.TempDir(), Version: "test", Models: []string{"openrouter/no-such-model"}})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("a refused model must not fail the open: %v", err)
	}
	status, note := bridge.Status()
	if status != StatusActive {
		t.Fatalf("a refused model must leave the session live: status=%q", status)
	}
	if !strings.Contains(note, "the provider has no such model") {
		t.Fatalf("the reader must read the agent's own sentence, got %q", note)
	}
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	// The agent's own list is what the panel shows, and it still says which model is in use.
	if got := modelInUse(t, session.State().Snapshot()); got != "default/model" {
		t.Fatalf("the session is on %q, want the model the agent opened with", got)
	}
	if err := session.Prompt(context.Background(), "una domanda"); err != nil {
		t.Fatalf("the conversation must still work on the agent's own model: %v", err)
	}
}

// A turn's span is on the thread the reader holds: running from the moment the prompt
// goes out until the ending is written, and only then closed. The per-turn flag and the
// page's `running` are the same fact seen from two sides, so the test asserts the thread
// rather than the field.
func TestATurnIsRunningFromThePromptToItsEnding(t *testing.T) {
	release := make(chan struct{})
	handler := func(agent *fakeAgent, method string, params json.RawMessage) any {
		if method == "session/prompt" {
			<-release
			return map[string]any{"stopReason": "end_turn"}
		}
		return agentVersion(1, "sess-1")(agent, method, params)
	}
	bridge, agent := activatedBridge(t, handler)
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatal("the bridge holds no session")
	}
	states := make(chan Thread, 16)
	session.State().Listen(func(thread Thread) { states <- thread })

	turn := make(chan error, 1)
	go func() { turn <- session.Prompt(context.Background(), "how big is the corpus?") }()
	waitForMethod(t, agent, "session/prompt")

	if !session.State().Snapshot().Running {
		t.Fatal("the thread does not say a turn is running while the agent works")
	}
	// The opening is announced, not only held: a browser that joined before the prompt
	// paints the working face from the snapshot it is sent.
	var sawRunning bool
	for drained := false; !drained; {
		select {
		case announced := <-states:
			sawRunning = sawRunning || announced.Running
		default:
			drained = true
		}
	}
	if !sawRunning {
		t.Fatal("no announced thread carried the turn in flight")
	}

	close(release)
	if err := <-turn; err != nil {
		t.Fatalf("the turn failed: %v", err)
	}
	thread := session.State().Snapshot()
	if thread.Running {
		t.Fatal("the thread still says a turn is running after it settled")
	}
	// The ending and the closed span travel together: the last snapshot the browser gets
	// carries both, so a turn is never finished but still marked running.
	if thread.Messages[0].Provisional {
		t.Fatal("the turn settled and the message it opened is still waiting")
	}
}
