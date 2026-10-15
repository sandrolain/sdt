package acpbridge

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// rawReply is what a handler returns to answer with a line the protocol does not
// allow (a malformed result, a body that is not JSON). The SDK ships an agent
// side, but driving it would mean stubbing thirteen interface methods per test and
// it cannot produce a malformed reply — and the newline-delimited framing is what
// the test has to prove anyway — so the fake speaks JSON-RPC 2.0 itself. The
// token %ID% is replaced by the request's own id.
type rawReply string

// rpcError is a JSON-RPC error response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handler answers one request. It receives the agent so it can emit notifications
// to the client before answering, which is how an agent streams a turn.
type handler func(agent *fakeAgent, method string, params json.RawMessage) any

// fakeAgent is a hand-written ACP peer on a real unix socket. It records every
// method it is asked and answers with whatever the handler returns.
type fakeAgent struct {
	t       *testing.T
	path    string
	ln      net.Listener
	handler handler

	mu       sync.Mutex
	calls    []string
	params   []json.RawMessage
	accepted int
	conn     net.Conn

	done chan struct{}
}

// newFakeAgent listens on a fresh unix socket and serves until the test ends.
func newFakeAgent(t *testing.T, h handler) *fakeAgent {
	t.Helper()
	path := socketPath(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	f := &fakeAgent{t: t, path: path, ln: ln, handler: h, done: make(chan struct{})}
	go f.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		<-f.done
	})
	return f
}

// socketPath returns a unix socket path short enough for the kernel: on macOS
// t.TempDir() plus a test name can exceed the 104-byte limit, and the bind then
// fails with an error that says nothing about why.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sdtacp")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "agent.sock")
}

// staleSocketPath returns a socket node nothing listens on, so a dial reaches the
// kernel and is refused rather than failing the policy's existence check.
func staleSocketPath(t *testing.T) string {
	t.Helper()
	path := socketPath(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	// Keep the node: the default unlinks it on close, and a dial to a missing
	// path is a policy refusal, not an unreachable agent.
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return path
}

func (f *fakeAgent) serve() {
	defer close(f.done)
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.accepted++
		f.mu.Unlock()
		go f.session(conn)
	}
}

// session reads newline-delimited JSON-RPC 2.0 and answers the requests.
func (f *fakeAgent) session(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

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
		f.record(msg.Method, msg.Params)
		if msg.ID == nil {
			// A notification: in ACP the client answers nothing.
			continue
		}
		line, err := answer(msg.ID, f.handler(f, msg.Method, msg.Params))
		if err != nil {
			return
		}
		f.writeLine(line)
	}
}

// notify emits a notification to the client, the way an agent streams a turn. It
// runs on the fake's own goroutine, so a failure is reported with t.Errorf rather
// than t.Fatalf, which may only be called from the test goroutine.
func (f *fakeAgent) notify(t *testing.T, method string, params any) {
	t.Helper()
	line, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		t.Errorf("marshal %s: %v", method, err)
		return
	}
	f.writeLine(line)
}

// writeLine writes one framed message to the client.
func (f *fakeAgent) writeLine(line []byte) {
	f.mu.Lock()
	conn := f.conn
	f.mu.Unlock()
	if conn == nil {
		return
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return
	}
}

func (f *fakeAgent) record(method string, params json.RawMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	f.params = append(f.params, params)
}

// methods lists the methods the agent was asked, in order.
func (f *fakeAgent) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// paramsFor decodes the params of the first call of a method.
func (f *fakeAgent) paramsFor(t *testing.T, method string) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, got := range f.calls {
		if got != method {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(f.params[i], &decoded); err != nil {
			t.Fatalf("params of %s: %v", method, err)
		}
		return decoded
	}
	t.Fatalf("the agent was never asked %s (asked: %v)", method, f.calls)
	return nil
}

// connections counts the connections the agent accepted, which is how a test
// observes that an inert bridge dialled nothing.
func (f *fakeAgent) connections() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accepted
}

// answer builds the reply line for a handler's return value.
func answer(id json.RawMessage, reply any) ([]byte, error) {
	if raw, ok := reply.(rawReply); ok {
		return []byte(strings.ReplaceAll(string(raw), "%ID%", string(id))), nil
	}
	envelope := map[string]any{"jsonrpc": "2.0", "id": id}
	if failure, ok := reply.(rpcError); ok {
		envelope["error"] = failure
	} else {
		envelope["result"] = reply
	}
	return json.Marshal(envelope)
}

// agentVersion answers initialize with the given protocol version and a session
// per session/new, which is the shape a well-behaved agent has.
func agentVersion(version int, sessionID string) handler {
	return func(_ *fakeAgent, method string, _ json.RawMessage) any {
		switch method {
		case "initialize":
			return map[string]any{
				"protocolVersion":   version,
				"agentCapabilities": map[string]any{},
				"authMethods":       []any{},
			}
		case "session/new":
			return map[string]any{"sessionId": sessionID}
		}
		return rpcError{Code: -32601, Message: "method not found: " + method}
	}
}
