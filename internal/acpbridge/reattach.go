package acpbridge

import (
	"context"
	"fmt"

	"github.com/coder/acp-go-sdk"
)

// lostContextNote is what a reader is told when the agent could not be put back on the
// conversation it had. The conversation itself is not lost — the thread is the viewer's
// and the panel keeps showing it — but the agent continues without the context it had
// built, and saying nothing would let a reader believe it remembers what it does not.
const lostContextNote = "the agent could not be put back on the conversation it had, so it continues without the context it built"

// restartState is what a death leaves behind: the conversations the dead process was
// carrying, and which of them a new browser would have been handed. It is taken before
// the dead connection is let go of, because a conversation belongs to its browser and
// outlives the process that was serving it.
type restartState struct {
	sessions []*Session
	initial  string
}

// takeRestartState lifts the conversations out of the registry and releases the waits
// the dead agent left — nobody can answer a permission an agent is no longer there to
// ask for — while keeping the conversations themselves: their threads are what the panel
// shows, and they are not the dead process's to take away.
func (b *Bridge) takeRestartState() *restartState {
	b.mu.Lock()
	state := &restartState{sessions: make([]*Session, 0, len(b.sessions)), initial: b.initial}
	for _, session := range b.sessions {
		state.sessions = append(state.sessions, session)
	}
	b.sessions = map[string]*Session{}
	b.initial = ""
	b.mu.Unlock()

	for _, session := range state.sessions {
		if cancelled := session.Close(); cancelled > 0 {
			b.log.Info("released the waits the dead agent left", "session", session.ID(), "waits", cancelled)
		}
	}
	return state
}

// carryOver puts the conversations a restart inherited back on the new connection, and
// returns the session a browser would be handed together with the sentence to show when
// an agent could not be put back on what it had.
func (b *Bridge) carryOver(ctx context.Context, conn *acp.ClientSideConnection, previous *restartState) (*Session, string, error) {
	var (
		initial *Session
		lost    bool
	)
	for i, session := range previous.sessions {
		wasInitial := session.ID() == previous.initial || (previous.initial == "" && i == 0)
		fresh, err := b.carryOne(ctx, conn, session)
		if err != nil {
			return nil, "", err
		}
		lost = lost || fresh
		if wasInitial {
			initial = session
		}
	}
	if initial == nil {
		return nil, "", fmt.Errorf("%w: a restart carries no conversation to take back", ErrHandshake)
	}
	b.mu.Lock()
	b.initial = initial.ID()
	b.mu.Unlock()

	note := ""
	if lost {
		note = lostContextNote
	}
	return initial, note, nil
}

// carryOne puts one conversation back on the new connection. The agent is asked to take
// its session back when it advertised that it can, and a fresh session is opened when it
// cannot or no longer holds it: either way the conversation keeps the thread the reader
// was looking at, so a restart reads as an interruption rather than as a new
// conversation. It reports whether the agent's own context had to be let go of.
func (b *Bridge) carryOne(ctx context.Context, conn *acp.ClientSideConnection, session *Session) (bool, error) {
	previous := session.ID()
	// The conversation goes back into the registry under the id it had *before* the load:
	// the updates a load replays name that session, and routing them is what lets the
	// thread decide whether to keep them.
	b.register(session, previous)

	if b.SupportsSessionLoad() && previous != "" {
		loadCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
		err := session.State().WhileLoading(func() error {
			return loadSession(loadCtx, conn, previous, b.root)
		})
		cancel()
		if err == nil {
			session.reattachTo(conn, previous)
			b.log.Info("the conversation was put back on the session the agent still holds", "session", previous)
			return false, nil
		}
		b.log.Warn("the agent could not take the session back; continuing on a new one",
			"session", previous, "error", err)
	}

	openCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	id, err := openSession(openCtx, conn, b.root)
	cancel()
	if err != nil {
		return false, err
	}
	session.reattachTo(conn, id)
	b.rekey(session, previous, id)
	return true, nil
}

// register puts a conversation in the registry under a session id.
func (b *Bridge) register(session *Session, id string) {
	b.mu.Lock()
	b.sessions[id] = session
	b.mu.Unlock()
}

// rekey moves a conversation from the session id it had to the one it now carries, so
// the registry never keeps an id nothing answers to.
func (b *Bridge) rekey(session *Session, from, to string) {
	b.mu.Lock()
	if b.sessions[from] == session {
		delete(b.sessions, from)
	}
	b.sessions[to] = session
	b.mu.Unlock()
}
