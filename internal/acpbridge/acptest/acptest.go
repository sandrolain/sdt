// Package acptest is a hand-written ACP peer for tests: newline-delimited JSON-RPC
// 2.0 over a real unix socket, answering what a test asks it to and emitting the
// notifications a turn produces.
//
// It lives in a package of its own rather than in a `_test.go` file because two
// levels of the codebase drive it — the bridge's own tests and the viewer's HTTP
// surface — and a second copy would drift from the first exactly where the framing
// matters. The SDK ships an agent side, but driving it would mean stubbing thirteen
// interface methods per test and it cannot send the malformed reply some tests need
// to cover.
package acptest

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// RawReply makes a handler answer with a line the protocol does not allow (a
// malformed result, a body that is not JSON). The token %ID% is replaced by the
// request's own id.
type RawReply string

// RPCError is a JSON-RPC error response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Handler answers one request. It receives the agent so it can emit notifications
// to the client before answering, which is how a real agent streams a turn.
type Handler func(agent *Agent, method string, params json.RawMessage) any

// Agent is the peer: it records every method it is asked and answers with whatever
// the handler returns.
type Agent struct {
	t       *testing.T
	path    string
	ln      net.Listener
	handler Handler

	mu       sync.Mutex
	calls    []string
	params   []json.RawMessage
	accepted int
	sessions int
	conn     io.ReadWriteCloser

	done chan struct{}
}

// NewAgent listens on a fresh unix socket under a short temp dir and serves until
// the test ends.
func NewAgent(t *testing.T, h Handler) *Agent {
	t.Helper()
	path := SocketPath(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	agent := &Agent{t: t, path: path, ln: ln, handler: h, done: make(chan struct{})}
	go agent.serve()
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Logf("agent listener close: %v", err)
		}
		<-agent.done
	})
	return agent
}

// Path is the socket the client should dial.
func (a *Agent) Path() string { return a.path }

// NewStream serves the handler over an already-connected pair of streams — the
// stdin and stdout of a spawned process — with the same framing loop NewAgent runs
// over a socket. The peer ends when the other side closes the stream; there is no
// listener, so there is nothing to accept.
func NewStream(t *testing.T, r io.Reader, w io.Writer, h Handler) *Agent {
	t.Helper()
	agent := &Agent{t: t, handler: h, done: make(chan struct{})}
	go func() {
		defer close(agent.done)
		agent.session(streamConn{reader: r, writer: w})
	}()
	t.Cleanup(func() { <-agent.done })
	return agent
}

// Done is closed when the peer has stopped serving.
func (a *Agent) Done() <-chan struct{} { return a.done }

// streamConn is a connection-shaped pair of streams: os.Stdin and os.Stdout are
// two files, and the session loop only ever reads and writes. Closing it is a
// no-op on purpose — the pipes belong to whoever created the process.
type streamConn struct {
	reader io.Reader
	writer io.Writer
}

func (c streamConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c streamConn) Write(p []byte) (int, error) { return c.writer.Write(p) }
func (c streamConn) Close() error                { return nil }

// SocketPath returns a unix socket path short enough for the kernel: on macOS
// t.TempDir() plus a test name can exceed the 104-byte limit, and the bind then
// fails with an error that says nothing about why. The directory is created under
// the process's own temp root.
func SocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sdtacp")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("socket dir removal: %v", err)
		}
	})
	return filepath.Join(dir, "agent.sock")
}

// StaleSocketPath returns a socket node nothing listens on, so a dial reaches the
// kernel and is refused rather than failing a policy's existence check.
func StaleSocketPath(t *testing.T) string {
	t.Helper()
	path := SocketPath(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	// Keep the node: the default unlinks it on close, and a dial to a missing path
	// is a policy refusal rather than an unreachable agent.
	if listener, ok := ln.(*net.UnixListener); ok {
		listener.SetUnlinkOnClose(false)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return path
}

func (a *Agent) serve() {
	defer close(a.done)
	for {
		conn, err := a.ln.Accept()
		if err != nil {
			return
		}
		a.mu.Lock()
		a.accepted++
		a.mu.Unlock()
		go a.session(conn)
	}
}

// session reads newline-delimited JSON-RPC 2.0 and answers the requests. It takes
// a connection-shaped pair rather than a net.Conn because the same loop serves a
// listening socket and the pipes of a spawned process; a second copy of the
// framing is exactly what this package exists to avoid.
func (a *Agent) session(conn io.ReadWriteCloser) {
	defer func() {
		if err := conn.Close(); err != nil {
			a.t.Logf("agent connection close: %v", err)
		}
	}()
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for reader.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(reader.Bytes(), &msg); err != nil {
			return
		}
		a.record(msg.Method, msg.Params)
		if msg.ID == nil {
			// A notification: in ACP the peer answers nothing.
			continue
		}
		line, err := answer(msg.ID, a.handler(a, msg.Method, msg.Params))
		if err != nil {
			return
		}
		a.writeLine(line)
	}
}

func (a *Agent) record(method string, params json.RawMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, method)
	a.params = append(a.params, params)
}

// Methods lists the methods the agent was asked, in order.
func (a *Agent) Methods() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.calls...)
}

// ParamsFor decodes the params of the first call of a method.
func (a *Agent) ParamsFor(t *testing.T, method string) map[string]any {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, asked := range a.calls {
		if asked != method {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(a.params[i], &decoded); err != nil {
			t.Fatalf("params of %s: %v", method, err)
		}
		return decoded
	}
	t.Fatalf("the agent was never asked %s (asked: %v)", method, a.calls)
	return nil
}

// NextSessionID hands out a distinct session id per session/new, the way a real
// agent does: two browsers on one connection get two conversations.
func (a *Agent) NextSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions++
	return "sess-" + strconv.Itoa(a.sessions)
}

// Connections counts the connections the agent accepted, which is how a test
// observes that an inert client dialled nothing.
func (a *Agent) Connections() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accepted
}

// Notify emits a notification to the client, the way an agent streams a turn. It
// runs on the agent's own goroutine, so a failure is reported with t.Errorf rather
// than t.Fatalf, which may only be called from the test goroutine.
func (a *Agent) Notify(t *testing.T, method string, params any) {
	t.Helper()
	line, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		t.Errorf("marshal %s: %v", method, err)
		return
	}
	a.writeLine(line)
}

// AgentVersion answers initialize with the given protocol version and hands out one
// session per session/new: the shape a well-behaved agent has.
func AgentVersion(version int, sessionID string) Handler {
	return func(_ *Agent, method string, _ json.RawMessage) any {
		switch method {
		case "initialize":
			return map[string]any{
				"protocolVersion":   version,
				"agentCapabilities": map[string]any{},
				"authMethods":       []any{},
			}
		case "session/new":
			return map[string]any{sessionIDKey: sessionID}
		}
		return RPCError{Code: -32601, Message: "method not found: " + method}
	}
}

// ConfigOption is one configuration option as a wire payload — the shape an agent sends
// in the answer to session/new and in a config_option update. It is exported as a map
// rather than a typed value because this package writes the wire, not the client's types.
func ConfigOption(id, category, current string, choices ...string) map[string]any {
	options := make([]map[string]any, 0, len(choices))
	for _, choice := range choices {
		options = append(options, map[string]any{"value": choice, "name": choice})
	}
	return map[string]any{
		"id":           id,
		"name":         id,
		"category":     category,
		"type":         "select",
		"currentValue": current,
		"options":      options,
	}
}

// ModelOption is the `model` config option an agent offers, which is how a model list
// reaches a client: a select whose current value is the model in use.
func ModelOption(current string, choices ...string) map[string]any {
	return ConfigOption("model", "model", current, choices...)
}

// OffersModels answers initialize, session/new and session/set_config_option the way an agent
// that offers models does: the list rides the answer to session/new and nothing announces it
// afterwards, which is the shape a real agent has, and a set is accepted by answering the
// option carrying the value now in use — the confirmation a client folds so the panel shows the
// agent's own answer rather than the choice it asked for. It is deliberately not folded into
// AgentVersion, so a test that is about the handshake keeps seeing a handshake and nothing else.
func OffersModels(current string, choices ...string) Handler {
	return func(agent *Agent, method string, params json.RawMessage) any {
		switch method {
		case newSessionMethod:
			return map[string]any{
				sessionIDKey:     agent.NextSessionID(),
				configOptionsKey: []map[string]any{ModelOption(current, choices...)},
			}
		case setConfigMethod:
			var request struct {
				Value string `json:"value"`
			}
			// A request this package cannot read is answered the way the protocol refuses a
			// malformed one: the test that sent it gets an error rather than a silent default.
			if err := json.Unmarshal(params, &request); err != nil {
				return RPCError{Code: -32602, Message: setConfigMethod + ": " + err.Error()}
			}
			return map[string]any{
				configOptionsKey: []map[string]any{ModelOption(request.Value, choices...)},
			}
		}
		return AgentVersion(acp.ProtocolVersionNumber, "sess-1")(agent, method, params)
	}
}

// The model's own two requests: the list arrives with the session's answer, and a choice goes
// back as a set. Named here because more than one handler answers them.
const (
	newSessionMethod = "session/new"
	setConfigMethod  = "session/set_config_option"
)

// The protocol's own field names, used by the handlers this package offers so their
// bodies read like the wire.
const (
	sessionIDKey     = "sessionId"
	updateKey        = "update"
	sessionUpdateKey = "sessionUpdate"
	contentKey       = "content"
	configOptionsKey = "configOptions"
)

// LastSessionID is the id handed out most recently.
func (a *Agent) LastSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions == 0 {
		return "sess-1"
	}
	return "sess-" + strconv.Itoa(a.sessions)
}

// Streams answers initialize and session/new, and answers session/prompt by
// emitting one agent message chunk before finishing the turn.
func Streams(text string) Handler {
	return func(agent *Agent, method string, _ json.RawMessage) any {
		switch method {
		case promptMethod:
			agent.Notify(agent.t, "session/update", map[string]any{
				sessionIDKey: agent.LastSessionID(),
				updateKey: map[string]any{
					sessionUpdateKey: "agent_message_chunk",
					contentKey:       textBlock(text),
				},
			})
			return map[string]any{"stopReason": "end_turn"}
		case "session/cancel":
			return nil
		case "session/new":
			return map[string]any{sessionIDKey: agent.NextSessionID()}
		}
		return AgentVersion(acp.ProtocolVersionNumber, "sess-1")(agent, method, nil)
	}
}

// promptMethod is the one method the two handlers below share with Streams, and the const
// exists because of that sharing: the turn is the method whose answer differs.
const promptMethod = "session/prompt"

// Refuses answers initialize and session/new and refuses the turn, which is what an agent
// does when the provider turns the request down: the turn comes back as a JSON-RPC error
// rather than as an answer.
func Refuses(code int, message string) Handler {
	streams := Streams("")
	return func(agent *Agent, method string, params json.RawMessage) any {
		if method == promptMethod {
			return RPCError{Code: code, Message: message}
		}
		return streams(agent, method, params)
	}
}

// Stops is Streams with the reason the turn ended on, so a turn that ran out of room or was
// refused can be told apart from one that simply finished.
func Stops(text, reason string) Handler {
	streams := Streams(text)
	return func(agent *Agent, method string, params json.RawMessage) any {
		reply := streams(agent, method, params)
		if method != promptMethod {
			return reply
		}
		result, ok := reply.(map[string]any)
		if !ok {
			return reply
		}
		result["stopReason"] = reason
		return result
	}
}

// writeLine writes one framed message to the client.
func (a *Agent) writeLine(line []byte) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn == nil {
		return
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return
	}
}

// answer builds the reply line for a handler's return value.
func answer(id json.RawMessage, reply any) ([]byte, error) {
	if raw, ok := reply.(RawReply); ok {
		return []byte(replaceID(string(raw), string(id))), nil
	}
	envelope := map[string]any{"jsonrpc": "2.0", "id": id}
	if failure, ok := reply.(RPCError); ok {
		envelope["error"] = failure
	} else {
		envelope["result"] = reply
	}
	return json.Marshal(envelope)
}

func replaceID(line, id string) string {
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		if i+4 <= len(line) && line[i:i+4] == "%ID%" {
			out = append(out, id...)
			i += 3
			continue
		}
		out = append(out, line[i])
	}
	return string(out)
}

// Loadable answers initialize with the loadSession capability, and answers
// session/load by replaying a two-message conversation for the session being loaded —
// the user's prompt and the agent's answer. That replay is what makes a duplicated
// history visible: an agent really does rebuild a conversation by sending its updates
// again, so a client that folds them onto the thread it already holds shows every
// message twice.
//
// With refuse it advertises the capability and refuses the load instead, which is what
// an agent that no longer holds the session does.
func Loadable(text string, refuse bool) Handler {
	streams := Streams(text)
	return func(agent *Agent, method string, params json.RawMessage) any {
		switch method {
		case "initialize":
			return map[string]any{
				"protocolVersion":   acp.ProtocolVersionNumber,
				"agentCapabilities": map[string]any{"loadSession": true},
				"authMethods":       []any{},
			}
		case "session/load":
			if refuse {
				return RPCError{Code: -32602, Message: "no such session"}
			}
			var request struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(params, &request); err != nil {
				return RPCError{Code: -32602, Message: "session/load: " + err.Error()}
			}
			agent.Notify(agent.t, "session/update", map[string]any{
				sessionIDKey: request.SessionID,
				updateKey: map[string]any{
					sessionUpdateKey: "user_message_chunk",
					contentKey:       textBlock("what I asked before"),
				},
			})
			agent.Notify(agent.t, "session/update", map[string]any{
				sessionIDKey: request.SessionID,
				updateKey: map[string]any{
					sessionUpdateKey: "agent_message_chunk",
					contentKey:       textBlock("and what you answered"),
				},
			})
			return map[string]any{}
		}
		return streams(agent, method, params)
	}
}

// textBlock builds a plain-text content block. It is the one content shape this package
// sends, so the wire's own spelling lives in one place rather than in every handler.
func textBlock(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}
