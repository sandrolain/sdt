// Package acpbridge owns the Agent Client Protocol (ACP) side of the viewer's
// chat surface: the dial policy for the socket the user supplies, the client
// callbacks an agent calls back into, the session lifecycle, and the thread state
// the browser renders.
//
// It is pure Go and imports no HTTP, so the viewer shell (`viewer/agent.go`) and a
// test drive the same domain. Its boundaries live here rather than in the caller.
//
// **The bridge never invents a target.** It reaches an agent in one of two ways: it
// dials an absolute unix socket path somebody named (Activate), or it starts the
// argv the project declared for itself (Start and EnsureStarted, which run it
// directly with no shell). A host and port, a relative path, a default and a
// remembered path are all refused, and nothing a browser sends can become a command:
// the viewer refuses a request that carries one, which is what keeps the spawn path
// out of a page's reach.
//
// **The process, once started, is the bridge's.** One agent per bridge, started
// idempotently, restarted within a bound when it dies while working, stopped when
// somebody asks for that, and taken down with the group it leads when the bridge
// closes — so a helper the agent spawned does not outlive the window that started it.
//
// **A conversation outlives the process that carried it.** The thread is the viewer's,
// and a restarted agent is put back on the session it still holds when it advertises
// that it can (SupportSessionLoad, session/load), with the replay of a history we
// already hold dropped rather than folded onto it.
//
// The bridge is inert until somebody asks for it: nothing is dialled, nothing is
// started, and no command is even resolved, before that. That is the property the
// whole trust story rests on, so it is a state of this type rather than a convention
// in the caller.
package acpbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
)

// Refusals, typed so a caller names a state instead of parsing a message. The
// first group is this bridge refusing by policy; the second is the agent failing
// to answer as the protocol requires.
var (
	// ErrNotActivated is returned when a command arrives before the bridge holds
	// a connection.
	ErrNotActivated = errors.New("acpbridge: the bridge is not activated")
	// ErrAlreadyActivated is returned when Activate is called on a live bridge.
	ErrAlreadyActivated = errors.New("acpbridge: the bridge is already activated")
	// ErrDialTarget is the dial policy's refusal: the target is not an absolute
	// unix socket path the user supplied.
	ErrDialTarget = errors.New("acpbridge: the dial target must be an absolute unix socket path")
	// ErrUnreachable is returned when the socket exists but nothing answers it.
	ErrUnreachable = errors.New("acpbridge: the agent is not reachable")
	// ErrHandshake is returned when initialize or session/new fails or answers
	// with a shape the protocol does not allow.
	ErrHandshake = errors.New("acpbridge: handshake failed")
	// ErrProtocolVersion is the version gate's refusal: the agent speaks a
	// protocol version this bridge does not implement. It fails closed.
	ErrProtocolVersion = errors.New("acpbridge: unsupported ACP protocol version")
	// ErrRefused is the callback policy's refusal, used by a callback this wave
	// does not serve.
	ErrRefused = errors.New("acpbridge: refused")
	// ErrPrompt is returned when the agent fails a turn it was asked to run.
	ErrPrompt = errors.New("acpbridge: the agent failed the turn")
	// ErrCancelled is returned when a pending request was cancelled rather than
	// answered — the browser that would have answered it went away.
	ErrCancelled = errors.New("acpbridge: the request was cancelled")
	// ErrTurnRunning is returned when a prompt arrives while the session is already
	// running one: the protocol runs a turn at a time, and two prompts would
	// interleave updates the thread cannot attribute.
	ErrTurnRunning = errors.New("acpbridge: a turn is already running")
)

// ClientName is the implementation name the bridge announces in initialize.
const ClientName = "sdtviewer"

// Options configures a bridge.
type Options struct {
	// Root is the directory the file callbacks are confined to, and the session's
	// workspace, so the agent's own relative paths and the confinement agree about
	// where "here" is.
	Root string
	// Version names the viewer build in the handshake, so the agent and its user
	// can tell which client is talking.
	Version string
	// Model is the id of the model the project declared for its agent. It is applied
	// to every session the bridge opens, so a reload or a restart does not silently
	// put the conversation back on the agent's own default. Empty means no
	// declaration, which is the agent's own choice of model.
	Model string
	// Log may be nil.
	Log *slog.Logger
}

// Bridge is one agent connection owned by one viewer process. It is inert until
// Activate succeeds and returns to inert when Close runs.
type Bridge struct {
	root    string
	version string
	// model is the declaration this bridge applies at every session open. It is set
	// once, when the bridge is built from the project's own file, and never changes:
	// the picker changes a session, not the project.
	model string
	perms *PermissionQueue
	log   *slog.Logger
	// trace is the sink the wire log writes into, set by the viewer before the
	// first start; rt is the capture of the run in flight, if any.
	trace TraceSink
	rt    *runTrace

	mu sync.Mutex
	// transport closes what the connection runs on: the socket the user named, or
	// the process the project declared. Keeping it here rather than in the caller
	// is what makes both transports end the same way.
	transport func() error
	conn      *acp.ClientSideConnection
	// loadSession is what the agent's initialize response said about session/load,
	// which is what lets a restart reattach instead of starting over.
	loadSession bool
	sessions    map[string]*Session
	initial     string

	// The supervisor's own state. startMu serialises a start — a caller's or a
	// restart's — so that "already started" cannot be read by two callers at once;
	// life is the bridge's life, which a process inherits and Close ends; proc,
	// cancel and intended describe the process that is running, and status is what
	// the panel shows.
	startMu     sync.Mutex
	life        context.Context
	halt        context.CancelFunc
	proc        *process
	cancel      context.CancelFunc
	intended    bool
	status      Status
	statusErr   string
	restarts    int
	restartFrom time.Time
}

// New returns an inert bridge with an empty session registry. It is its own sink:
// one connection can carry several browser sessions, so what the callbacks publish
// is routed by the session the update names.
func New(opts Options) *Bridge {
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	bridge := &Bridge{
		root:     opts.Root,
		version:  opts.Version,
		model:    opts.Model,
		log:      log,
		sessions: map[string]*Session{},
		status:   StatusIdle,
	}
	// The life a started process inherits: Close ends it, so nothing the bridge
	// started can outlive the bridge.
	bridge.life, bridge.halt = context.WithCancel(context.Background())
	bridge.perms = NewPermissionQueue(bridge.Permission)
	return bridge
}

var _ Sink = (*Bridge)(nil)

// SessionUpdate routes one update to the thread of the session it names. An update
// for a session that closed is dropped: a browser that went away must not leave
// the connection dead behind it.
func (b *Bridge) SessionUpdate(ctx context.Context, notification acp.SessionNotification) error {
	session, ok := b.Session(string(notification.SessionId))
	if !ok {
		return nil
	}
	return session.state.SessionUpdate(ctx, notification)
}

// Permission shows the card on the session that asked for it.
func (b *Bridge) Permission(ctx context.Context, request Permission) {
	if session, ok := b.Session(request.SessionID); ok {
		session.state.Permission(ctx, request)
	}
}

// Session returns the live session with this ACP id.
func (b *Bridge) Session(id string) (*Session, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	session, ok := b.sessions[id]
	return session, ok
}

// NewSession opens an ACP session on the live connection, with its own thread.
func (b *Bridge) NewSession(ctx context.Context) (*Session, error) {
	b.mu.Lock()
	conn := b.conn
	b.mu.Unlock()
	if conn == nil {
		return nil, ErrNotActivated
	}
	session, note, err := b.newSession(ctx, conn)
	if err != nil {
		return nil, err
	}
	// A browser that opened this session is the reader the sentence is for, and the note
	// channel is the one it reads: nothing in the command that opened the session carries
	// a sentence, and the session is usable either way.
	b.setNote(note)
	return session, nil
}

// newSession opens one session on an already-live connection and registers it. The
// attach path opens its session here and so does a second browser, so a
// conversation always begins the same way. The sentence it returns is what a reader is
// owed about the session that just opened — empty when there is nothing to say.
func (b *Bridge) newSession(ctx context.Context, conn *acp.ClientSideConnection) (*Session, string, error) {
	openCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	id, options, err := openSession(openCtx, conn, b.root)
	if err != nil {
		return nil, "", err
	}
	session := &Session{
		id:    id,
		conn:  conn,
		state: NewThreadState(id),
		perms: b.perms,
		ctx:   ctx,
	}
	// What the agent offers for this session arrives with the answer that opened it, and
	// nowhere else: folded here, so a browser that joins a session the bridge just opened
	// finds the model list in the first state it receives.
	session.state.ConfigOptions(options)
	// The detach closes over the session rather than over the id it was built with: a
	// reattach can give the conversation a new id, and the registry must lose the key it
	// actually holds.
	session.detach = func() { b.forget(session) }
	b.mu.Lock()
	b.sessions[id] = session
	b.mu.Unlock()
	// The declaration goes on the session it is about, and the bring-up's own bound covers
	// it: an agent that takes the open and then answers nothing to the set is an agent the
	// caller is not left waiting on.
	return session, b.applyDeclaredModel(openCtx, session), nil
}

// modelConfigID is the option an agent names its model with — the protocol's own category
// for it, and the id a declared model is set through. It is the one config id the bridge
// knows by name; every other option is the panel's to show and the reader's to set.
const modelConfigID = "model"

// applyDeclaredModel puts the model the project declared on a session that has just come up,
// and returns the sentence a reader is owed when the agent would not take it — the empty
// string when there is nothing to say. A refusal is not a failure of the open: the session
// runs, on the model the agent chose, and only the reader's expectation needs correcting.
// The declaration is the user's, so a refusal is not retried: the next session open applies
// it again.
func (b *Bridge) applyDeclaredModel(ctx context.Context, session *Session) string {
	if b.model == "" {
		return ""
	}
	if err := session.SetConfig(ctx, modelConfigID, b.model); err != nil {
		b.log.Warn("the agent did not take the declared model", "model", b.model, "error", err)
		return fmt.Sprintf("the agent would not take the declared model %q: %s",
			b.model, RefusalSentence(err, "the model"))
	}
	return ""
}

// forget removes a conversation from the registry under whatever id it carries now. A
// reattach that had to open a fresh session gives its conversation a new id, so deleting
// the id it was built with would leave a closed conversation registered — reachable by a
// command nobody is there to answer.
func (b *Bridge) forget(session *Session) {
	id := session.ID()
	b.mu.Lock()
	if b.sessions[id] == session {
		delete(b.sessions, id)
	}
	if b.initial == id {
		b.initial = ""
	}
	b.mu.Unlock()
}

// CloseSessions closes every session of this connection and reports how many.
func (b *Bridge) CloseSessions() int {
	b.mu.Lock()
	sessions := make([]*Session, 0, len(b.sessions))
	for _, session := range b.sessions {
		sessions = append(sessions, session)
	}
	b.sessions = map[string]*Session{}
	b.initial = ""
	b.mu.Unlock()
	for _, session := range sessions {
		session.Close()
	}
	return len(sessions)
}

// Permissions is the queue a browser answers through, in the order the agent
// asked.
func (b *Bridge) Permissions() *PermissionQueue { return b.perms }

// Activated reports whether the bridge holds a handshaken agent connection.
func (b *Bridge) Activated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conn != nil
}

// SessionID is the session the handshake opened, and empty while the bridge is
// inert. It is the session a browser is handed when it arrives; a second browser
// opens its own with NewSession.
func (b *Bridge) SessionID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.initial
}

// Activate dials the socket at socketPath and joins it to the bridge: it refuses a
// target the dial policy rejects, fails closed on a protocol version this bridge
// does not speak, and is one of the two paths by which a connection comes into
// existence — the other is Start, which spawns the process the project declared.
// It returns the ACP session id.
func (b *Bridge) Activate(ctx context.Context, socketPath string) (string, error) {
	sock, err := dialSocket(ctx, socketPath)
	if err != nil {
		return "", err
	}
	session, _, err := b.attach(ctx, sock, func() error { return sock.Close() }, nil,
		traceRun{transport: "socket", socket: socketPath})
	if err != nil {
		return "", err
	}
	b.log.Info("agent bridge activated", "session", session.ID(), "socket", socketPath)
	return session.ID(), nil
}

// attach joins a transport to the bridge: it builds the one connection over the
// transport's pair, completes the handshake, and opens the session a browser is
// handed. Both transports end here — the socket a user named and the process a
// project declared — so the client callbacks, the file confinement, the permission
// queue, the thread fold and the session registry have a single code path instead
// of two that can drift apart.
//
// A failure closes the transport and leaves the bridge inert, so the next attempt
// starts clean. run carries the identity of the wire-log run this attach is: the
// transport and what the project declared on it, which the capture's header
// records and whose writer opens once the session id completes it.
func (b *Bridge) attach(ctx context.Context, rw io.ReadWriteCloser, closeTransport func() error, previous *restartState, run traceRun) (*Session, string, error) {
	b.mu.Lock()
	live := b.conn != nil
	b.mu.Unlock()
	if live {
		return nil, "", b.abandon(closeTransport, ErrAlreadyActivated)
	}

	// The capture's seam: the wrapped rw is what the SDK reads and writes, so
	// every frame of both directions crosses it, for both transports, and the
	// protocol itself is never touched.
	rt := b.beginTrace(run)
	if rt != nil {
		rw = &tracedConn{inner: rw, rt: rt}
	}

	// The connection's callbacks are the policy, not a pass-through, and the bridge
	// is their sink: this is the one place the two are joined.
	callbacks := &client{root: b.root, sink: b, perms: b.perms}
	conn := acp.NewClientSideConnection(callbacks, rw, rw)
	conn.SetLogger(b.log)

	// The handshake is bounded: an agent that accepts the transport and then answers
	// nothing would otherwise hang the start forever, because the SDK drops a line
	// it cannot parse and the request it belonged to never completes.
	handshakeCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	res, err := handshake(handshakeCtx, conn, b.version)
	cancel()
	if err != nil {
		b.traceFail(rt, err)
		return nil, "", b.abandon(closeTransport, err)
	}

	b.mu.Lock()
	b.conn, b.loadSession = conn, res.AgentCapabilities.LoadSession
	b.mu.Unlock()

	// The session the browser is handed when it arrives. A first start opens a fresh
	// one; a restart carries over what the dead process left, which is where a
	// conversation survives an agent that died.
	var (
		session *Session
		note    string
	)
	if previous == nil {
		session, note, err = b.newSession(ctx, conn)
	} else {
		session, note, err = b.carryOver(ctx, conn, previous)
	}
	if err != nil {
		b.mu.Lock()
		b.conn, b.loadSession = nil, false
		b.mu.Unlock()
		b.traceFail(rt, err)
		return nil, "", b.abandon(closeTransport, err)
	}

	// The session id completes the run: the capture's writer opens under it, and
	// the buffered prefix — the header and whatever crossed during the handshake —
	// is flushed into the file the id named.
	if rt != nil {
		rt.setSession(session.ID())
	}

	b.mu.Lock()
	b.transport, b.initial = closeTransport, session.ID()
	// A successful attach leaves a live agent, whichever transport carried it: the process
	// view is set here so the socket path and the spawned path cannot report different
	// states for the same fact.
	b.status, b.statusErr = StatusActive, note
	b.mu.Unlock()
	return session, note, nil
}

// abandon closes a transport whose attach did not complete, keeping the caller's
// error and logging the close, so a leaked socket or process is visible rather
// than silent.
func (b *Bridge) abandon(closeTransport func() error, attachErr error) error {
	if err := closeTransport(); err != nil {
		b.log.Debug("closing the transport after a failed attach", "error", err)
	}
	return attachErr
}

// SetTrace installs the sink the wire log writes into, or nil to switch it off.
// It is a setter rather than an Options field for two reasons: the viewer opens
// the file at the first activation — a path that cannot be written is a
// sentence where a reader is looking, not a viewer that refuses to start — and
// a restart reattaches inside the bridge, which then asks the same sink again
// for the new run's writer.
func (b *Bridge) SetTrace(sink TraceSink) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trace = sink
}

// beginTrace starts the wire-log run for an attach: it builds the run's capture
// and writes the header line, which the capture holds in its prefix until the
// session id opens the writer. It returns nil when the project declared no
// capture, which is the ordinary case.
func (b *Bridge) beginTrace(run traceRun) *runTrace {
	b.mu.Lock()
	sink := b.trace
	b.mu.Unlock()
	if sink == nil {
		return nil
	}
	// The run starts now; the header says so, and the same instant names a
	// {timestamp} file — the session id that completes the run arrives later, so
	// the timestamp is taken once, here, and both readers share it.
	run.started = time.Now().UTC()
	rt := newRunTrace(sink, run)
	rt.emit(traceEntry{
		T:         run.started.Format(time.RFC3339Nano),
		Run:       "header",
		Version:   b.version,
		Transport: run.transport,
		Command:   run.command,
		Socket:    run.socket,
		PID:       run.pid,
	})
	b.mu.Lock()
	b.rt = rt
	b.mu.Unlock()
	return rt
}

// traceFail ends a run whose attach did not complete. Whatever crossed before
// the failure is still the reader's to see, so the capture opens under the
// nosession name, writes the close marker carrying the cause, and ends.
func (b *Bridge) traceFail(rt *runTrace, cause error) {
	if rt == nil {
		return
	}
	rt.fail(cause)
	b.mu.Lock()
	if b.rt == rt {
		b.rt = nil
	}
	b.mu.Unlock()
	if err := rt.failure(); err != nil {
		b.log.Debug("the wire log stopped", "error", err)
	}
}

// SupportsSessionLoad reports whether the connected agent advertised session/load,
// which is the one thing a restart needs to put a conversation back on the session
// it had instead of starting a new one.
func (b *Bridge) SupportsSessionLoad() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loadSession
}

// Close ends the connection and returns the bridge to inert: every session's
// pending waits are cancelled, the transport goes (a socket, or a process whose
// group is signalled and waited for), and the bridge's own life ends, so nothing it
// started outlives it. It is safe to call on an inert bridge.
func (b *Bridge) Close() error {
	err := b.teardown()
	b.halt()
	return err
}

// teardown closes the transport and forgets the process and the connection without
// ending the bridge's life, which is what lets Stop leave a bridge that can start
// again while Close leaves one that cannot.
func (b *Bridge) teardown() error {
	b.CloseSessions()
	b.mu.Lock()
	transport, conn := b.transport, b.conn
	// The wire-log run in flight ends with the connection it captured: the close
	// marker carries whatever the bridge last had to say — the exit of a dead
	// process, the refusal that stopped it — and "closed" when it has nothing
	// beyond the stop itself.
	rt, traceReason := b.rt, b.statusErr
	// The cycle's state — its process, the cancel that ends it and whether its exit was
	// intended — is deliberately kept: a stop that arrives while a restart is scheduled
	// reaches the cancel, and the flag is what a death racing a stop is read by. Both are
	// cleared where a new cycle begins.
	b.proc = nil
	b.transport, b.conn, b.loadSession, b.initial, b.rt = nil, nil, false, "", nil
	b.mu.Unlock()

	if conn == nil {
		return nil
	}
	var err error
	if transport != nil {
		err = transport()
	}
	if rt != nil {
		if traceReason == "" {
			traceReason = "closed"
		}
		rt.finish(traceReason)
		if ferr := rt.failure(); ferr != nil {
			b.log.Debug("the wire log stopped", "error", ferr)
		}
	}
	b.log.Info("agent bridge closed")
	return err
}

// Root is the directory the file callbacks are confined to.
func (b *Bridge) Root() string { return b.root }
