package acpbridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// Session is one browser connection's conversation: the ACP session it opened and
// the thread it accumulates. One browser connection owns one session; the viewer
// opens a session when a browser arrives and closes it when the browser leaves.
type Session struct {
	// mu guards id and conn, which a reattach replaces: the conversation outlives the
	// process that was carrying it, and only what it runs on changes.
	mu     sync.RWMutex
	id     string
	conn   *acp.ClientSideConnection
	state  *ThreadState
	perms  *PermissionQueue
	detach func()
	// ctx is the life of the conversation: it ends when the browser that opened it
	// goes away, and every turn of this session runs under it, so an agent's work
	// dies with the connection that asked for it.
	ctx context.Context

	turnsMu sync.Mutex
	running bool
}

// ID is the ACP session id the agent assigned.
func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// connection is the connection this conversation runs on and the id it carries there.
// A reattach replaces both together, so they are read together.
func (s *Session) connection() (*acp.ClientSideConnection, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn, s.id
}

// reattachTo points this conversation at a connection that carries it: the process
// behind the session changed, the conversation did not. A new id is given when the
// agent could not take the old one back, which is a fresh session with the thread the
// reader already has.
func (s *Session) reattachTo(conn *acp.ClientSideConnection, id string) {
	s.mu.Lock()
	s.conn, s.id = conn, id
	s.mu.Unlock()
}

// Context is the life of this conversation. A caller that starts a turn without a
// more specific context uses this one, so the turn ends with the browser rather than
// with the request that asked for it.
func (s *Session) Context() context.Context { return s.ctx }

// State is the thread this session accumulates.
func (s *Session) State() *ThreadState { return s.state }

// Prompt sends the user's text and returns when the turn ends. It blocks for the
// whole turn, so a caller that also serves the browser's cancel has to call it
// from its own goroutine — which is what the viewer's command endpoint does.
//
// One turn at a time: a second prompt arrives as ErrTurnRunning rather than as two
// interleaved streams of updates that the thread could not attribute to either.
func (s *Session) Prompt(ctx context.Context, text string) error {
	s.turnsMu.Lock()
	if s.running {
		s.turnsMu.Unlock()
		return ErrTurnRunning
	}
	s.running = true
	s.turnsMu.Unlock()
	defer func() {
		s.turnsMu.Lock()
		s.running = false
		s.turnsMu.Unlock()
	}()

	conn, sessionID := s.connection()
	id := s.state.openUserMessage(text)
	response, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(sessionID),
		MessageId: &id,
		Prompt:    []acp.ContentBlock{acp.TextBlock(text)},
	})
	if err != nil {
		return fmt.Errorf("acpbridge: prompt: %w", err)
	}
	s.state.foldUsage(response.Usage)
	s.state.notify()
	return nil
}

// Cancel asks the agent to stop the current turn. The protocol makes it a
// notification, so it returns as soon as it is written and the agent answers by
// finishing the turn it is in.
func (s *Session) Cancel(ctx context.Context) error {
	conn, sessionID := s.connection()
	if err := conn.Cancel(ctx, acp.CancelNotification{SessionId: acp.SessionId(sessionID)}); err != nil {
		return fmt.Errorf("acpbridge: cancel: %w", err)
	}
	return nil
}

// Close ends this conversation and returns how many permission waits it cancelled:
// a card nobody can answer must not leave the agent blocked, so a disconnect
// cancels every wait of this session. The ACP session itself is not closed — the
// protocol's optional session/close would need a capability this wave does not ask
// for, and the connection dies with the viewer anyway.
func (s *Session) Close() int {
	cancelled := s.perms.CancelSession(s.ID())
	if s.detach != nil {
		s.detach()
	}
	return cancelled
}
