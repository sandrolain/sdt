package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// command posts one command envelope the way the panel does.
func command(t *testing.T, ts *httptest.Server, envelope any) *http.Response {
	t.Helper()
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	return post(t, ts, agentCommandPath, string(body), ts.URL)
}

// activate dials the fake agent through the endpoint the user's panel would use.
func activate(t *testing.T, ts *httptest.Server, socket string) {
	t.Helper()
	response := post(t, ts, agentActivatePath, `{"socket":"`+socket+`"}`, ts.URL)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("activation answered %d", response.StatusCode)
	}
}

// streamReader is one open SSE response, read frame by frame.
type streamReader struct {
	response *http.Response
	scanner  *bufio.Scanner
}

// openStream opens the stream with the origin a browser sends. A refusal is an
// ordinary response, so the caller checks the status before reading frames.
func openStream(t *testing.T, ts *httptest.Server, origin string) *streamReader {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, ts.URL+agentStreamPath, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &streamReader{response: response, scanner: scanner}
}

// next reads the next state frame. It blocks until one arrives; a stream that ended
// fails the test rather than hanging it.
func (s *streamReader) next(t *testing.T) acpbridge.StateEnvelope {
	t.Helper()
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue // a comment or the blank line between frames
		}
		var envelope acpbridge.StateEnvelope
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &envelope); err != nil {
			t.Fatalf("state frame: %v (%s)", err, line)
		}
		return envelope
	}
	t.Fatalf("the stream ended: %v", s.scanner.Err())
	return acpbridge.StateEnvelope{}
}

// nextUntil reads frames until one satisfies the predicate.
func (s *streamReader) nextUntil(t *testing.T, what string, ok func(acpbridge.StateEnvelope) bool) acpbridge.StateEnvelope {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state := s.next(t); ok(state) {
			return state
		}
	}
	t.Fatalf("no frame arrived %s", what)
	return acpbridge.StateEnvelope{}
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
	if s.agent.Activated() {
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

// The stream refuses before activation and for a foreign origin, and so does the
// command endpoint.
func TestAgentStreamAndCommandRefuseBeforeActivationAndCrossOrigin(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))

	for _, tc := range []struct {
		name   string
		origin string
	}{
		{"a foreign origin", "http://someone-else.test"},
		{"no origin at all", ""},
	} {
		t.Run("stream "+tc.name, func(t *testing.T) {
			if response := openStream(t, ts, tc.origin); response.response.StatusCode != http.StatusForbidden {
				t.Fatalf("the stream answered %d, want 403", response.response.StatusCode)
			}
		})
		t.Run("command "+tc.name, func(t *testing.T) {
			response := post(t, ts, agentCommandPath, `{"type":"cancel"}`, tc.origin)
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("the command answered %d, want 403", response.StatusCode)
			}
		})
	}

	// Before activation both are refused with the activation reason, not the origin.
	if response := openStream(t, ts, ts.URL); response.response.StatusCode != http.StatusForbidden {
		t.Fatalf("the stream before activation answered %d", response.response.StatusCode)
	}
	if response := command(t, ts, acpbridge.CancelEnvelope{Type: acpbridge.EnvelopeCancel}); response.StatusCode != http.StatusForbidden {
		t.Fatalf("the command before activation answered %d", response.StatusCode)
	}

	activate(t, ts, agent.Path())
	if !s.agent.Activated() {
		t.Fatal("the bridge is not activated")
	}
	// A command with no stream open yet has no session to reach.
	if response := command(t, ts, acpbridge.CancelEnvelope{Type: acpbridge.EnvelopeCancel}); response.StatusCode != http.StatusConflict {
		t.Fatalf("a command with no stream answered %d, want 409", response.StatusCode)
	}
}

// A turn over the bridge: the prompt goes up as a POST, the thread comes down the
// stream whole.
func TestAgentStreamRelaysAPromptAndTheThread(t *testing.T) {
	_, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("the corpus has 224 analyses"))
	activate(t, ts, agent.Path())

	stream := openStream(t, ts, ts.URL)
	first := stream.next(t)
	if first.Type != acpbridge.EnvelopeState || first.SessionID == "" {
		t.Fatalf("the first frame is %+v", first)
	}
	if len(first.Thread.Messages) != 0 {
		t.Fatalf("a fresh session carries %d messages", len(first.Thread.Messages))
	}

	response := command(t, ts, acpbridge.PromptEnvelope{
		Type:      acpbridge.EnvelopePrompt,
		Text:      "how big is the corpus?",
		SessionID: first.SessionID,
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the prompt answered %d", response.StatusCode)
	}

	state := stream.nextUntil(t, "holding the agent's answer", func(state acpbridge.StateEnvelope) bool {
		return len(state.Thread.Messages) == 2 && state.Thread.Messages[1].Parts[0].Text == "the corpus has 224 analyses"
	})
	if state.Thread.Messages[0].Role != acpbridge.RoleUser || state.Thread.Messages[0].Parts[0].Text != "how big is the corpus?" {
		t.Fatalf("the user's message is %+v", state.Thread.Messages[0])
	}
	if state.SessionID != first.SessionID {
		t.Fatalf("the stream jumped from session %q to %q", first.SessionID, state.SessionID)
	}
}

// A cancel reaches the agent, and a prompt sent before any stream is refused rather
// than silently dropped.
func TestAgentCommandDeliversCancel(t *testing.T) {
	_, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	stream := openStream(t, ts, ts.URL)
	frame := stream.next(t)
	if response := command(t, ts, acpbridge.CancelEnvelope{
		Type:      acpbridge.EnvelopeCancel,
		SessionID: frame.SessionID,
	}); response.StatusCode != http.StatusOK {
		t.Fatalf("the cancel answered %d", response.StatusCode)
	}

	deadline := time.Now().Add(5 * time.Second)
	for !containsMethod(agent.Methods(), "session/cancel") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !containsMethod(agent.Methods(), "session/cancel") {
		t.Fatalf("the agent was asked %v, want a session/cancel", agent.Methods())
	}
}

// One session per browser stream: the stream gets its own, not the one activation
// opened.
func TestAgentStreamOwnsItsOwnSession(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())
	initial := s.agent.SessionID()

	stream := openStream(t, ts, ts.URL)
	state := stream.next(t)
	if state.SessionID == initial {
		t.Fatalf("the browser joined the session activation opened (%q)", initial)
	}
	if _, ok := s.agent.Session(state.SessionID); !ok {
		t.Fatalf("the browser's session %q is not registered", state.SessionID)
	}
	// A command that names nothing reaches the stream's session, which is the
	// single-browser case.
	if response := command(t, ts, acpbridge.CancelEnvelope{Type: acpbridge.EnvelopeCancel}); response.StatusCode != http.StatusOK {
		t.Fatalf("a command with no session id answered %d", response.StatusCode)
	}
}

// The permission relay: the card is resolved in place for the browser, and the
// agent's wait is released with the answer.
func TestAgentPermissionAnswerReleasesTheAgent(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	stream := openStream(t, ts, ts.URL)
	sessionID := stream.next(t).SessionID

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

	card := stream.nextUntil(t, "carrying the permission card", func(state acpbridge.StateEnvelope) bool {
		return findPermission(state) != nil
	})
	permission := findPermission(card)
	if permission.ToolKind != "edit" || permission.Decision != "" {
		t.Fatalf("the card is %+v", permission)
	}

	response := command(t, ts, acpbridge.PermissionEnvelope{
		Type:      acpbridge.EnvelopePermission,
		RequestID: permission.RequestID,
		Allow:     true,
		SessionID: sessionID,
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the answer answered %d", response.StatusCode)
	}
	if !<-answered {
		t.Fatal("the agent's wait was released with a denial")
	}
	stream.nextUntil(t, "showing the decision", func(state acpbridge.StateEnvelope) bool {
		resolved := findPermission(state)
		return resolved != nil && resolved.Decision == acpbridge.DecisionAllowed
	})
}

// Deactivate returns the server to inert and ends the open streams.
func TestAgentDeactivateReturnsToInert(t *testing.T) {
	s, ts := newAgentViewer(t)
	agent := acptest.NewAgent(t, acptest.Streams("hello"))
	activate(t, ts, agent.Path())

	stream := openStream(t, ts, ts.URL)
	stream.next(t)

	if response := post(t, ts, agentDeactivatePath, "", ts.URL); response.StatusCode != http.StatusOK {
		t.Fatalf("deactivate answered %d", response.StatusCode)
	}
	if s.agent.Activated() {
		t.Fatal("the bridge is still activated")
	}
	// The stream is ended rather than left showing a dead thread: drain it, skipping
	// the keep-alive comments SSE carries, and it has to stop without another state.
	ended := make(chan bool, 1)
	go func() {
		for stream.scanner.Scan() {
			if strings.HasPrefix(stream.scanner.Text(), "data: ") {
				ended <- false
				return
			}
		}
		ended <- true
	}()
	select {
	case closed := <-ended:
		if !closed {
			t.Fatal("the stream kept feeding after deactivate")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream was not ended by deactivate")
	}

	// And it can be activated again: the panel's own loop.
	activate(t, ts, agent.Path())
	if !s.agent.Activated() {
		t.Fatal("the bridge did not come back")
	}
	reopened := openStream(t, ts, ts.URL)
	if reopened.response.StatusCode != http.StatusOK {
		t.Fatalf("the stream after reactivation answered %d", reopened.response.StatusCode)
	}
}

// A non-loopback bind disables the surface, as decision 0027 disables document
// editing, and says so.
func TestAgentSurfaceIsDisabledOffLoopback(t *testing.T) {
	s, err := newServer(t.TempDir())
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
	if stream := openStream(t, ts, ts.URL); stream.response.StatusCode != http.StatusForbidden {
		t.Fatalf("a disabled surface answered a stream with %d", stream.response.StatusCode)
	}
}

// Malformed input is refused one request at a time, and a refusal is never fatal.
func TestAgentEndpointsRefuseMalformedInput(t *testing.T) {
	_, ts := newAgentViewer(t)
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
	stream := openStream(t, ts, ts.URL)
	stream.next(t)

	if response := post(t, ts, agentCommandPath, "not an envelope", ts.URL); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a garbage command answered %d", response.StatusCode)
	}
	if response := post(t, ts, agentCommandPath, `{"type":"something-else"}`, ts.URL); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("an unknown command kind answered %d", response.StatusCode)
	}

	// The conversation is still there: a prompt after the refusals still streams.
	if response := command(t, ts, acpbridge.PromptEnvelope{Type: acpbridge.EnvelopePrompt, Text: "still there?"}); response.StatusCode != http.StatusOK {
		t.Fatalf("the prompt after the refusals answered %d", response.StatusCode)
	}
	stream.nextUntil(t, "holding the answer after the refusals", func(state acpbridge.StateEnvelope) bool {
		return len(state.Thread.Messages) == 2
	})
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

func containsMethod(methods []string, method string) bool {
	for _, asked := range methods {
		if asked == method {
			return true
		}
	}
	return false
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

// A browser may leave Origin off a same-origin request; the one header a page cannot
// forge is what decides, and a cross-site fetch marker is refused whatever the Origin
// says.
func TestAgentOriginRule(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		site   string
		want   bool
	}{
		{"the viewer's own origin", "http://127.0.0.1:8791", "", true},
		{"a foreign origin", "http://someone-else.test", "", false},
		{"no origin, a same-origin fetch", "", "same-origin", true},
		{"no origin at all", "", "", false},
		{"no origin, a cross-site fetch", "", "cross-site", false},
		{"matching origin but a cross-site marker", "http://127.0.0.1:8791", "cross-site", false},
		{"a URL scheme that is not http", "ftp://127.0.0.1:8791", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/agent/command", nil)
			request.Host = "127.0.0.1:8791"
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			if tc.site != "" {
				request.Header.Set("Sec-Fetch-Site", tc.site)
			}
			if got := agentOrigin(request); got != tc.want {
				t.Fatalf("agentOrigin = %v, want %v", got, tc.want)
			}
		})
	}
}
