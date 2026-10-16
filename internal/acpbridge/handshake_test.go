package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

func testBridge(t *testing.T) *Bridge {
	t.Helper()
	return New(Options{Root: t.TempDir(), Version: "test-version"})
}

// The happy path, and the shape the agent sees: the bridge announces the protocol
// version it speaks, the capabilities it serves, and opens one session on the
// project root.
func TestActivateCompletesTheHandshake(t *testing.T) {
	agent := newFakeAgent(t, agentVersion(acp.ProtocolVersionNumber, "sess-1"))
	bridge := testBridge(t)

	session, err := bridge.Activate(context.Background(), agent.Path())
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if session != "sess-1" {
		t.Fatalf("Activate returned session %q, want sess-1", session)
	}
	if !bridge.Activated() {
		t.Fatal("Activate succeeded and the bridge reports itself inert")
	}
	if got := bridge.SessionID(); got != "sess-1" {
		t.Fatalf("SessionID() = %q, want sess-1", got)
	}
	if got, want := agent.Methods(), []string{"initialize", "session/new"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("the agent was asked %v, want %v", got, want)
	}

	initialize := agent.ParamsFor(t, "initialize")
	if got := initialize["protocolVersion"]; got != float64(acp.ProtocolVersionNumber) {
		t.Fatalf("initialize protocolVersion = %v, want %d", got, acp.ProtocolVersionNumber)
	}
	capabilities, ok := initialize["clientCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("initialize carried no clientCapabilities: %v", initialize)
	}
	fs, ok := capabilities["fs"].(map[string]any)
	if !ok || fs["readTextFile"] != true || fs["writeTextFile"] != true {
		t.Fatalf("the bridge did not advertise the file callbacks it serves: %v", capabilities)
	}
	// The terminal is refused by policy, so it is not advertised: an agent has no
	// reason to offer one, and if it does anyway the refusal stands.
	if capabilities["terminal"] == true {
		t.Fatalf("the bridge advertised terminal support it refuses: %v", capabilities)
	}

	newsSession := agent.ParamsFor(t, "session/new")
	if got := newsSession["cwd"]; got != bridge.Root() {
		t.Fatalf("session/new cwd = %v, want the project root %q", got, bridge.Root())
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if bridge.Activated() || bridge.SessionID() != "" {
		t.Fatal("Close left the bridge activated")
	}
	// Closing is safe on an inert bridge, and a closed bridge can be activated
	// again: the viewer's deactivate/activate loop depends on it.
	if err := bridge.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate after Close: %v", err)
	}
}

// The agent's model list is announced once, in the answer that opens the session, and
// nowhere else: a bridge that dropped that answer would show a reader a control with
// nothing in it until an update a real agent never sends. So the fold happens where the
// answer arrives, and a browser joining the session finds it in the first state it gets.
func TestTheOpenAnswerFoldsTheAgentConfigIntoTheThread(t *testing.T) {
	agent := newFakeAgent(t, offeringAgent("openrouter/meta/muse-spark-open",
		"openrouter/meta/muse-spark-open", "opencode/deepseek-v4.1-flash-free", "opencode/big-pickle"))
	bridge := testBridge(t)
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	config := session.State().Snapshot().Config
	if len(config) != 1 {
		t.Fatalf("the thread carries %d config options, want the one the open answered with: %v", len(config), config)
	}
	var option struct {
		ID           string `json:"id"`
		Category     string `json:"category"`
		Type         string `json:"type"`
		CurrentValue string `json:"currentValue"`
		Options      []struct {
			Value string `json:"value"`
		} `json:"options"`
	}
	if err := json.Unmarshal(config[0], &option); err != nil {
		t.Fatalf("the folded option is not the agent's own payload: %v (%s)", err, config[0])
	}
	if option.ID != "model" || option.Category != "model" || option.Type != "select" {
		t.Fatalf("the folded option lost its identity: %+v", option)
	}
	if option.CurrentValue != "openrouter/meta/muse-spark-open" {
		t.Fatalf("currentValue = %q, want the model the agent opened the session with", option.CurrentValue)
	}
	if len(option.Options) != 3 {
		t.Fatalf("the folded option carries %d choices, want the 3 the agent sent", len(option.Options))
	}
}

// An agent that offers nothing leaves an empty config rather than an invented one, and the
// fold never clears a list a session already holds — the empty answer is the degrade case,
// not a reason to take a reader's list away.
func TestAnAgentWithoutConfigOptionsLeavesTheThreadsConfigEmpty(t *testing.T) {
	agent := newFakeAgent(t, agentVersion(acp.ProtocolVersionNumber, "sess-1"))
	bridge := testBridge(t)
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	if config := session.State().Snapshot().Config; len(config) != 0 {
		t.Fatalf("a session with no config option carries %v, want none", config)
	}
}

// A version this bridge does not speak fails closed, and no session is opened:
// the negotiation is the one place a future protocol cannot pass silently.
func TestActivateRefusesAFutureProtocolVersion(t *testing.T) {
	cases := []struct {
		name    string
		version int
	}{
		{"the agent speaks v2", acp.ProtocolVersionNumber + 1},
		{"the agent answers with no version at all", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := newFakeAgent(t, agentVersion(tc.version, "sess-1"))
			bridge := testBridge(t)

			_, err := bridge.Activate(context.Background(), agent.Path())
			if !errors.Is(err, ErrProtocolVersion) {
				t.Fatalf("Activate = %v, want ErrProtocolVersion", err)
			}
			if bridge.Activated() {
				t.Fatal("a failed handshake left the bridge activated")
			}
			if got := agent.Methods(); len(got) != 1 || got[0] != "initialize" {
				t.Fatalf("the agent was asked %v, want initialize only", got)
			}
		})
	}
}

// Everything that can go wrong after the socket connects is ErrHandshake, so the
// viewer names one state for it; the agent never sees a session.
func TestActivateHandshakeFailures(t *testing.T) {
	cases := []struct {
		name    string
		handler handler
	}{
		{"initialize is refused by the agent", func(_ *fakeAgent, method string, _ json.RawMessage) any {
			if method == "initialize" {
				return rpcError{Code: -32602, Message: "bad params"}
			}
			return map[string]any{"sessionId": "sess-1"}
		}},
		{"initialize answers with a malformed result", func(_ *fakeAgent, method string, _ json.RawMessage) any {
			if method == "initialize" {
				return rawReply(`{"jsonrpc":"2.0","id":%ID%,"result":{"protocolVersion":"one"}}`)
			}
			return map[string]any{"sessionId": "sess-1"}
		}},
		{"session/new is refused by the agent", func(_ *fakeAgent, method string, _ json.RawMessage) any {
			if method == "session/new" {
				return rpcError{Code: -32000, Message: "no session for you"}
			}
			return map[string]any{"protocolVersion": acp.ProtocolVersionNumber}
		}},
		{"session/new returns no session id", agentVersion(acp.ProtocolVersionNumber, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := newFakeAgent(t, tc.handler)
			bridge := testBridge(t)
			_, err := bridge.Activate(context.Background(), agent.Path())
			if !errors.Is(err, ErrHandshake) {
				t.Fatalf("Activate = %v, want ErrHandshake", err)
			}
			if bridge.Activated() {
				t.Fatal("a failed handshake left the bridge activated")
			}
		})
	}
}

// A body the client cannot parse is not a reply at all: the SDK drops it and the
// request it belonged to would wait forever, so the bridge's own deadline is what
// turns it into a named failure.
func TestActivateBoundsTheHandshake(t *testing.T) {
	agent := newFakeAgent(t, func(_ *fakeAgent, _ string, _ json.RawMessage) any {
		return rawReply("this is not json")
	})
	bridge := testBridge(t)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := bridge.Activate(ctx, agent.Path())
	if !errors.Is(err, ErrHandshake) {
		t.Fatalf("Activate = %v, want ErrHandshake", err)
	}
	if bridge.Activated() {
		t.Fatal("a timed-out handshake left the bridge activated")
	}
}

// One connection per bridge: a second Activate is refused rather than replacing
// the live session behind the browser's back.
func TestActivateIsRefusedOnALiveBridge(t *testing.T) {
	agent := newFakeAgent(t, agentVersion(acp.ProtocolVersionNumber, "sess-1"))
	bridge := testBridge(t)
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := bridge.Activate(context.Background(), agent.Path()); !errors.Is(err, ErrAlreadyActivated) {
		t.Fatalf("second Activate = %v, want ErrAlreadyActivated", err)
	}
}

// The bridge's own root is what the session and the callbacks share, so a test
// pins that Activate does not invent a directory.
func TestRootIsTheSessionWorkspace(t *testing.T) {
	agent := newFakeAgent(t, agentVersion(acp.ProtocolVersionNumber, "sess-7"))
	bridge := New(Options{Root: "/tmp/project-root", Version: "test-version"})
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if got := agent.ParamsFor(t, "session/new")["cwd"]; got != "/tmp/project-root" {
		t.Fatalf("session/new cwd = %v, want /tmp/project-root", got)
	}
}
