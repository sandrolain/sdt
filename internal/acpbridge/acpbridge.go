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
	// Sink receives what the browser must see. Nil discards it.
	Sink Sink
	// Log may be nil.
	Log *slog.Logger
}

// Bridge is one agent connection owned by one viewer process. It is inert until
// Activate succeeds and returns to inert when Close runs.
type Bridge struct {
	root    string
	version string
	sink    Sink
	perms   *PermissionQueue
	log     *slog.Logger

	mu        sync.Mutex
	socket    net.Conn
	conn      *acp.ClientSideConnection
	sessionID string
}

// New returns an inert bridge. A caller that takes no session updates may leave
// Sink nil; the permission queue is always live, because a permission request
// arrives whether or not the surface is ready to show it.
func New(opts Options) *Bridge {
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	bridge := &Bridge{root: opts.Root, version: opts.Version, sink: opts.Sink, log: log}
	bridge.perms = NewPermissionQueue(func(ctx context.Context, request Permission) {
		if opts.Sink != nil {
			opts.Sink.Permission(ctx, request)
		}
	})
	return bridge
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

// SessionID is the ACP session the handshake opened, and empty while the bridge
// is inert.
func (b *Bridge) SessionID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessionID
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
	// The connection's callbacks are the policy, not a pass-through: this is the
	// one place the two are joined.
	callbacks := &client{root: b.root, sink: b.sink, perms: b.perms}
	conn := acp.NewClientSideConnection(callbacks, sock, sock)
	conn.SetLogger(b.log)

	// The handshake is bounded: an agent that accepts the socket and then answers
	// nothing would otherwise hang the activation forever, because the SDK drops a
	// line it cannot parse and the request it belonged to never completes.
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	sessionID, err := handshake(ctx, conn, b.root, b.version)
	if err != nil {
		// A handshake that did not complete leaves no connection behind, so the
		// bridge stays inert and the next Activate starts clean.
		if closeErr := sock.Close(); closeErr != nil {
			b.log.Debug("closing the socket after a failed handshake", "error", closeErr)
		}
		return "", err
	}

	b.mu.Lock()
	b.socket, b.conn, b.sessionID = sock, conn, sessionID
	b.mu.Unlock()
	b.log.Info("agent bridge activated", "session", sessionID, "socket", socketPath)
	return sessionID, nil
}

// Close ends the connection and returns the bridge to inert. It is safe to call
// on an inert bridge.
func (b *Bridge) Close() error {
	b.mu.Lock()
	sock, conn := b.socket, b.conn
	b.socket, b.conn, b.sessionID = nil, nil, ""
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
