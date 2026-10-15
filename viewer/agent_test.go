package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sandrolain/sdt/internal/acpbridge"
	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

// newAgentViewer builds the viewer over a temp root, with the bridge available as a
// loopback bind makes it.
func newAgentViewer(t *testing.T) (*server, *httptest.Server) {
	t.Helper()
	s, err := newServer(t.TempDir())
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	s.enableAgent("127.0.0.1")
	ts := httptest.NewServer(s.mux())
	t.Cleanup(ts.Close)
	return s, ts
}

// post sends a viewer request carrying the server's own origin, as a browser does.
func post(t *testing.T, ts *httptest.Server, path, body, origin string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// activate dials the fake agent through the endpoint the user's panel would use.
func activate(t *testing.T, ts *httptest.Server, socket string) {
	t.Helper()
	response := post(t, ts, agentActivatePath, `{"socket":"`+socket+`"}`, ts.URL)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("activation answered %d", response.StatusCode)
	}
}

// dialAgent opens the browser side of the bridge, carrying the origin a browser
// always sends. The response is returned so a test can assert a refusal.
func dialAgent(t *testing.T, ts *httptest.Server, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + agentWSPath
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
}

// readState reads the next whole-thread state from the bridge.
func readState(t *testing.T, conn *websocket.Conn) acpbridge.StateEnvelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("the bridge sent a %v message", typ)
	}
	var envelope acpbridge.StateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("state envelope: %v (%s)", err, data)
	}
	return envelope
}

// readUntil reads states until one satisfies the predicate.
func readUntil(t *testing.T, conn *websocket.Conn, what string, ok func(acpbridge.StateEnvelope) bool) acpbridge.StateEnvelope {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state := readState(t, conn); ok(state) {
			return state
		}
	}
	t.Fatalf("no state arrived %s", what)
	return acpbridge.StateEnvelope{}
}

// writeCommand sends one browser command.
func writeCommand(t *testing.T, conn *websocket.Conn, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write command: %v", err)
	}
}

// The state endpoint is the only one reachable before activation, and an inert
// bridge has dialled nothing.
func TestAgentStateIsReachableAndInert(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))

	response, err := ts.Client().Get(ts.URL + agentStatePath)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("state answered %d", response.StatusCode)
	}
	var state agentStateResponse
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatalf("state body: %v", err)
	}
	if state.Activated || state.Connected || state.Session != "" || state.Error != "" {
		t.Fatalf("an inert bridge reported %+v", state)
	}
	if got := agent.Connections(); got != 0 {
		t.Fatalf("the agent accepted %d connections before activation", got)
	}
	if !s.agent.Activated() == false {
		t.Fatal("the bridge reports itself activated")
	}
}

// A cross-origin activation, and one with no origin at all, are both refused: the
// bridge's only intended caller is the panel in the user's own page.
func TestAgentActivationRefusesAForeignOrigin(t *testing.T) {
	_, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))

	cases := []struct {
		name   string
		origin string
	}{
		{"a foreign origin", "http://someone-else.test"},
		{"no origin at all", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := post(t, ts, agentActivatePath, `{"socket":"`+agent.Path()+`"}`, tc.origin)
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("the activation answered %d, want 403", response.StatusCode)
			}
		})
	}
	if got := agent.Connections(); got != 0 {
		t.Fatalf("a refused activation dialled the agent %d times", got)
	}
}

// The happy path, and the state the browser then reads.
func TestAgentActivationDialsTheUsersSocket(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	if got, want := agent.Methods(), []string{"initialize", "session/new"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("the agent was asked %v", got)
	}
	response, err := ts.Client().Get(ts.URL + agentStatePath)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var state agentStateResponse
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatalf("state body: %v", err)
	}
	if !state.Activated || !state.Connected || state.Session == "" {
		t.Fatalf("the activated state is %+v", state)
	}
	if state.Session != s.agent.SessionID() {
		t.Fatalf("the state names session %q, the bridge holds %q", state.Session, s.agent.SessionID())
	}
}

// The dial policy, not just the happy path: a host and port is refused with a
// message the user can act on.
func TestAgentActivationRefusesATcpTarget(t *testing.T) {
	_, ts := newAgentViewer(t)
	response := post(t, ts, agentActivatePath, `{"socket":"127.0.0.1:9000"}`, ts.URL)
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("a host:port activation answered %d, want 502", response.StatusCode)
	}
	body, err := readBody(response)
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	if !strings.Contains(body, "absolute unix socket path") {
		t.Fatalf("the refusal reads %q", body)
	}
}

// The upgrade refuses before activation and for a foreign origin.
func TestAgentWSRefusesBeforeActivationAndCrossOrigin(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))

	if _, response, err := dialAgent(t, ts, ts.URL); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("the upgrade before activation was not refused (%v, %v)", err, response)
	}

	activate(t, ts, agent.Path())
	if _, response, err := dialAgent(t, ts, "http://someone-else.test"); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("the cross-origin upgrade was not refused (%v, %v)", err, response)
	}
	// The bridge is up, so the refusal was the origin rule and not the activation.
	if !s.agent.Activated() {
		t.Fatal("the bridge is not activated")
	}
}

// A turn over the bridge: the prompt goes up, the thread comes down whole.
func TestAgentWSRelaysAPromptAndTheThread(t *testing.T) {
	_, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("the corpus has 224 analyses"))
	activate(t, ts, agent.Path())

	conn, _, err := dialAgent(t, ts, ts.URL)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	first := readState(t, conn)
	if first.Type != acpbridge.EnvelopeState || first.SessionID == "" {
		t.Fatalf("the first state is %+v", first)
	}
	if len(first.Thread.Messages) != 0 {
		t.Fatalf("a fresh session carries %d messages", len(first.Thread.Messages))
	}

	writeCommand(t, conn, acpbridge.PromptEnvelope{Type: acpbridge.EnvelopePrompt, Text: "how big is the corpus?"})
	state := readUntil(t, conn, "holding the agent's answer", func(state acpbridge.StateEnvelope) bool {
		return len(state.Thread.Messages) == 2 && state.Thread.Messages[1].Parts[0].Text == "the corpus has 224 analyses"
	})
	if state.Thread.Messages[0].Role != acpbridge.RoleUser || state.Thread.Messages[0].Parts[0].Text != "how big is the corpus?" {
		t.Fatalf("the user's message is %+v", state.Thread.Messages[0])
	}
	if state.SessionID != first.SessionID {
		t.Fatalf("the state jumped from session %q to %q", first.SessionID, state.SessionID)
	}
}

// One session per browser connection: the socket that arrives gets its own, not the
// one activation opened.
func TestAgentWSOwnsItsOwnSession(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())
	initial := s.agent.SessionID()

	conn, _, err := dialAgent(t, ts, ts.URL)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	state := readState(t, conn)
	if state.SessionID == initial {
		t.Fatalf("the browser joined the session activation opened (%q)", initial)
	}
	if _, ok := s.agent.Session(state.SessionID); !ok {
		t.Fatalf("the browser's session %q is not registered", state.SessionID)
	}
}

// The permission relay: the card is resolved in place for the browser, and the
// agent's wait is released with the answer.
func TestAgentWSPermissionAnswerReleasesTheAgent(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	conn, _, err := dialAgent(t, ts, ts.URL)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	state := readState(t, conn)
	sessionID := state.SessionID

	// What the agent does when it needs the user: it asks, and it waits.
	answered := make(chan bool, 1)
	go func() {
		allow, err := s.agent.Permissions().Ask(context.Background(), acpbridge.Permission{
			SessionID:  sessionID,
			ToolCallID: "call-1",
			ToolKind:   "edit",
			Title:      "Write context/notes/x.md",
			Input:      map[string]any{"path": "context/notes/x.md"},
		})
		if err != nil {
			answered <- false
			return
		}
		answered <- allow
	}()

	card := readUntil(t, conn, "carrying the permission card", func(state acpbridge.StateEnvelope) bool {
		return findPermission(state) != nil
	})
	permission := findPermission(card)
	if permission.ToolKind != "edit" || permission.Decision != "" {
		t.Fatalf("the card is %+v", permission)
	}

	writeCommand(t, conn, acpbridge.PermissionEnvelope{
		Type:      acpbridge.EnvelopePermission,
		RequestID: permission.RequestID,
		Allow:     true,
	})
	if !<-answered {
		t.Fatal("the agent's wait was released with a denial")
	}
	readUntil(t, conn, "showing the decision", func(state acpbridge.StateEnvelope) bool {
		resolved := findPermission(state)
		return resolved != nil && resolved.Decision == acpbridge.DecisionAllowed
	})
}

// Deactivate returns the server to inert and closes the open browsers.
func TestAgentDeactivateReturnsToInert(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	conn, _, err := dialAgent(t, ts, ts.URL)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	readState(t, conn)

	if response := post(t, ts, agentDeactivatePath, "", ts.URL); response.StatusCode != http.StatusOK {
		t.Fatalf("deactivate answered %d", response.StatusCode)
	}
	if s.agent.Activated() {
		t.Fatal("the bridge is still activated")
	}
	// The browser's socket is closed rather than left showing a dead thread.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("the browser connection stayed open after deactivate")
	}

	// And it can be activated again: the panel's own loop.
	activate(t, ts, agent.Path())
	if !s.agent.Activated() {
		t.Fatal("the bridge did not come back")
	}
	if _, response, err := dialAgent(t, ts, ts.URL); err != nil || response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("the upgrade after reactivation failed (%v, %v)", err, response)
	}
}

// A non-loopback bind disables the surface, as decision 0027 disables document
// editing, and says so.
func TestAgentSurfaceIsDisabledOffLoopback(t *testing.T) {
	root := t.TempDir()
	s, err := newServer(root)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	s.enableAgent("0.0.0.0")
	ts := httptest.NewServer(s.mux())
	t.Cleanup(ts.Close)

	response, err := ts.Client().Get(ts.URL + agentStatePath)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var state agentStateResponse
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatalf("state body: %v", err)
	}
	if state.Error == "" {
		t.Fatalf("a disabled surface reported %+v", state)
	}
	if refused := post(t, ts, agentActivatePath, `{"socket":"/tmp/none.sock"}`, ts.URL); refused.StatusCode != http.StatusForbidden {
		t.Fatalf("a disabled surface answered an activation with %d", refused.StatusCode)
	}
}

// findPermission returns the first permission card of a thread.
func findPermission(state acpbridge.StateEnvelope) *acpbridge.Permission {
	for _, message := range state.Thread.Messages {
		for _, part := range message.Parts {
			if part.Permission != nil {
				return part.Permission
			}
		}
	}
	return nil
}

func readBody(response *http.Response) (string, error) {
	defer func() { _ = response.Body.Close() }()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Error, nil
}

// Malformed input is refused one message at a time, and a refusal is never fatal:
// the socket carries a closed vocabulary and a broken message does not end a
// conversation.
func TestAgentEndpointsRefuseMalformedInput(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))

	if response := post(t, ts, agentActivatePath, "{", ts.URL); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a malformed activation body answered %d", response.StatusCode)
	}
	request, err := http.NewRequest(http.MethodGet, ts.URL+agentActivatePath, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Origin", ts.URL)
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatalf("get activation: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("a GET activation answered %d", response.StatusCode)
	}

	activate(t, ts, agent.Path())
	conn, _, err := dialAgent(t, ts, ts.URL)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	readState(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte("not a command at all")); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, []byte{0x00}); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	writeCommand(t, conn, acpbridge.CancelEnvelope{Type: acpbridge.EnvelopeCancel})

	// The conversation is still there: the cancel reached the agent, and a prompt
	// after it still streams.
	deadline := time.Now().Add(5 * time.Second)
	for !containsMethod(agent.Methods(), "session/cancel") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !containsMethod(agent.Methods(), "session/cancel") {
		t.Fatalf("the agent was asked %v, want a session/cancel", agent.Methods())
	}
	writeCommand(t, conn, acpbridge.PromptEnvelope{Type: acpbridge.EnvelopePrompt, Text: "still there?"})
	readUntil(t, conn, "holding the answer after the garbage", func(state acpbridge.StateEnvelope) bool {
		return len(state.Thread.Messages) == 2
	})
	_ = s
}

// The small helpers, each branch, because a wrong map from a typed error to a
// sentence is a wrong sentence to the user.
func TestAgentHelperBranches(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a refused dial target", acpbridge.ErrDialTarget, errAgentPath},
		{"an unreachable agent", acpbridge.ErrUnreachable, errAgentUnreachable},
		{"a future protocol version", acpbridge.ErrProtocolVersion, errAgentVersion},
		{"a failed handshake", acpbridge.ErrHandshake, errAgentHandshake},
	}
	for _, tc := range cases {
		if got := activationMessage(tc.err); got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}

	// The newest state wins when the writer has not taken the previous one: the
	// payload is the whole thread, so the older state is redundant, not lost.
	out := make(chan []byte, 1)
	pushLatest(out, []byte("first"))
	pushLatest(out, []byte("second"))
	if got := string(<-out); got != "second" {
		t.Fatalf("the writer would send %q", got)
	}

	if agentDecision(true) != acpbridge.DecisionAllowed || agentDecision(false) != acpbridge.DecisionDenied {
		t.Fatal("the answer does not map onto the card's vocabulary")
	}
}

// An unknown command kind is a refusal rather than a silent no-op.
func TestRunAgentCommandRefusesAnUnknownKind(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())
	session, ok := s.agent.Session(s.agent.SessionID())
	if !ok {
		t.Fatal("the bridge holds no session")
	}
	if err := s.runAgentCommand(context.Background(), nil, session, acpbridge.Command{Kind: "nope"}); err == nil {
		t.Fatal("an unknown command kind was accepted")
	}
}

func containsMethod(methods []string, method string) bool {
	for _, asked := range methods {
		if asked == method {
			return true
		}
	}
	return false
}
