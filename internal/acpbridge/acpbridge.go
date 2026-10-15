// Package acpbridge owns the Agent Client Protocol (ACP) side of the viewer's
// chat surface: the dial policy for the socket the user supplies, the client
// callbacks an agent calls back into, the session lifecycle, and the thread state
// the browser renders.
//
// It is pure Go and imports no HTTP, so the viewer shell (`viewer/agent.go`) and a
// test drive the same domain. Two boundaries are absolute and live here: **the
// bridge dials, it never spawns**, and a dial target is an absolute unix socket
// path the user named at activation time — no host and port, no relative path, no
// default, no discovery, no remembered path.
//
// The bridge is inert until Activate: nothing is dialled, and no socket is even
// resolved, before it. That is the property the whole trust story rests on, so it
// is a state of this type rather than a convention in the caller.
package acpbridge

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

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
	// Log may be nil.
	Log *slog.Logger
}

// Bridge is one agent connection owned by one viewer process. It is inert until
// Activate succeeds and returns to inert when Close runs.
type Bridge struct {
	root    string
	version string
	perms   *PermissionQueue
	log     *slog.Logger

	mu       sync.Mutex
	socket   net.Conn
	conn     *acp.ClientSideConnection
	sessions map[string]*Session
	initial  string
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
		log:      log,
		sessions: map[string]*Session{},
	}
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
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	id, err := openSession(ctx, conn, b.root)
	if err != nil {
		return nil, err
	}
	session := &Session{
		id:    id,
		conn:  conn,
		state: NewThreadState(id),
		perms: b.perms,
		detach: func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.sessions, id)
			if b.initial == id {
				b.initial = ""
			}
		},
	}
	b.mu.Lock()
	b.sessions[id] = session
	b.mu.Unlock()
	return session, nil
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

// Activate dials the socket at socketPath and completes the handshake: it refuses
// a target the dial policy rejects, fails closed on a protocol version this
// bridge does not speak, and is the only path by which a connection comes into
// existence. It returns the ACP session id.
func (b *Bridge) Activate(ctx context.Context, socketPath string) (string, error) {
	b.mu.Lock()
	if b.conn != nil {
		b.mu.Unlock()
		return "", ErrAlreadyActivated
	}
	b.mu.Unlock()

	sock, err := dialSocket(ctx, socketPath)
	if err != nil {
		return "", err
	}
	// The connection's callbacks are the policy, not a pass-through, and the bridge
	// is their sink: this is the one place the two are joined.
	callbacks := &client{root: b.root, sink: b, perms: b.perms}
	conn := acp.NewClientSideConnection(callbacks, sock, sock)
	conn.SetLogger(b.log)

	// The handshake is bounded: an agent that accepts the socket and then answers
	// nothing would otherwise hang the activation forever, because the SDK drops a
	// line it cannot parse and the request it belonged to never completes.
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	err = handshake(ctx, conn, b.version)
	if err != nil {
		// A handshake that did not complete leaves no connection behind, so the
		// bridge stays inert and the next Activate starts clean.
		if closeErr := sock.Close(); closeErr != nil {
			b.log.Debug("closing the socket after a failed handshake", "error", closeErr)
		}
		return "", err
	}
	b.mu.Lock()
	b.socket, b.conn = sock, conn
	b.mu.Unlock()

	// The session the browser is handed when it arrives. A second browser opens its
	// own; the first is opened here so activation leaves a usable conversation.
	session, err := b.NewSession(ctx)
	if err != nil {
		if closeErr := sock.Close(); closeErr != nil {
			b.log.Debug("closing the socket after a failed session", "error", closeErr)
		}
		b.mu.Lock()
		b.socket, b.conn = nil, nil
		b.mu.Unlock()
		return "", err
	}
	b.mu.Lock()
	b.initial = session.ID()
	b.mu.Unlock()
	b.log.Info("agent bridge activated", "session", session.ID(), "socket", socketPath)
	return session.ID(), nil
}

// Close ends the connection and returns the bridge to inert: every session's
// pending waits are cancelled, then the socket goes. It is safe to call on an
// inert bridge.
func (b *Bridge) Close() error {
	b.CloseSessions()
	b.mu.Lock()
	sock, conn := b.socket, b.conn
	b.socket, b.conn = nil, nil
	b.mu.Unlock()

	if conn == nil {
		return nil
	}
	var err error
	if sock != nil {
		err = sock.Close()
	}
	b.log.Info("agent bridge closed")
	return err
}

// Root is the directory the file callbacks are confined to.
func (b *Bridge) Root() string { return b.root }
