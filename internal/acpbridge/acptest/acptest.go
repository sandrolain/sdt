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
	conn     net.Conn

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

// session reads newline-delimited JSON-RPC 2.0 and answers the requests.
func (a *Agent) session(conn net.Conn) {
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

// sessionIDKey is the protocol's own field name, used by the handlers this package
// offers so their bodies read like the wire.
const sessionIDKey = "sessionId"

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
		case "session/prompt":
			agent.Notify(agent.t, "session/update", map[string]any{
				sessionIDKey: agent.LastSessionID(),
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]any{"type": "text", "text": text},
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
