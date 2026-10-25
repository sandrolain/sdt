package acpbridge

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// Session is the bridge's conversation: the ACP session it opened and the thread it
// accumulates. One bridge holds one conversation, every stream joins it, and it ends only
// when the reader asks for a new one, stops the agent, or the bridge closes — a browser
// leaving is not an act on it.
type Session struct {
	// mu guards id and conn, which a reattach replaces: the conversation outlives the
	// process that was carrying it, and only what it runs on changes.
	mu     sync.RWMutex
	id     string
	conn   *acp.ClientSideConnection
	state  *ThreadState
	perms   *PermissionQueue
	elicits *ElicitationQueue
	detach  func()
	// ctx is the life of the conversation, derived from the bridge's own life rather
	// than from a request: every turn runs under it, so a browser that closes its panel
	// does not call off the work it asked for. End is what stops it.
	ctx  context.Context
	halt context.CancelFunc

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
		// Cleared after the ending was written, so a reader who sees the turn finish also
		// sees it stopped: the two facts travel in one snapshot, never in two.
		s.state.SetRunning(false)
	}()

	// The span opens before the message it belongs to: a reader who sees the turn is
	// running, and the prompt that opened it, from the same side of the wire.
	s.state.SetRunning(true)

	conn, sessionID := s.connection()
	id := s.state.openUserMessage(text)
	response, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(sessionID),
		MessageId: &id,
		Prompt:    []acp.ContentBlock{acp.TextBlock(text)},
	})
	if err != nil {
		// The reader is told before the error leaves: the returned error is the viewer's
		// log and its tests, and the thread is where the reader looks.
		s.state.failTurn(id, turnFailure(err))
		return fmt.Errorf("acpbridge: prompt: %w", err)
	}
	s.state.foldUsage(response.Usage)
	if reason := reportedStop(response.StopReason); reason != "" {
		s.state.endTurn(id, reason)
	} else {
		// The turn is over even when its ending explains nothing: the reader is not still
		// waiting on an answer that has already arrived.
		s.state.settle(id)
	}
	s.state.notify()
	return nil
}

// turnFailure is what the reader is told about a turn that failed: the agent's own sentence,
// unwrapped from the JSON-RPC envelope it travels in, and the error's own text for anything
// that is not that shape (a transport that went away, say).
func turnFailure(err error) string { return RefusalSentence(err, "the turn") }

// RefusalSentence is how a refusal is put to a reader: the agent's own words for it, with the
// JSON-RPC envelope they travel in taken off, and — when the agent gave no words at all — the
// code, named by what was turned down. Nothing is reworded, because a refusal is the agent's
// to explain and the viewer cannot know better, and a raw envelope is not something a reader
// should have to look at. It is exported because both halves read it from here: the bridge's
// own note and the viewer's answer to a command are one sentence, and neither paraphrases the
// other.
func RefusalSentence(err error, refused string) string {
	var request *acp.RequestError
	if errors.As(err, &request) {
		if request.Message != "" {
			return request.Message
		}
		if request.Code != 0 {
			return fmt.Sprintf("the agent refused %s (code %d)", refused, request.Code)
		}
	}
	return err.Error()
}

// reportedStop returns the stop reason worth telling the reader, and the empty string for the
// endings that need no explanation: a turn that simply ended, and one the reader themselves
// cancelled, are already understood from the thread.
func reportedStop(reason acp.StopReason) string {
	switch reason {
	case acp.StopReasonMaxTokens, acp.StopReasonMaxTurnRequests, acp.StopReasonRefusal:
		return string(reason)
	default:
		return ""
	}
}

// SetConfig switches one of this conversation's configuration options — the model it runs on,
// its mode — and folds the agent's own answer into the thread, because what the agent says is
// in use is the truth the panel shows: a control whose value came from the request would be the
// browser's guess rather than the agent's word. The protocol's setter takes a value id, which
// is what a select carries. The caller passes its own context, the way Cancel does: a browser
// that went away takes its own set with it, and no timeout is invented here.
func (s *Session) SetConfig(ctx context.Context, configID, value string) error {
	conn, sessionID := s.connection()
	answer, err := conn.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{
			SessionId: acp.SessionId(sessionID),
			ConfigId:  acp.SessionConfigId(configID),
			Value:     acp.SessionConfigValueId(value),
		},
	})
	if err != nil {
		return fmt.Errorf("acpbridge: session/set_config_option: %w", err)
	}
	s.state.ConfigOptions(answer.ConfigOptions)
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

// Close releases the waits this conversation holds without ending it: a restart lifts the
// conversation out of a dead process and puts it back on the new one, and the waits the
// dead agent left are the only thing that has to go. It reports how many waits it
// cancelled.
func (s *Session) Close() int {
	cancelled := s.perms.CancelSession(s.ID()) + s.elicits.CancelSession(s.ID())
	if s.detach != nil {
		s.detach()
	}
	return cancelled
}

// End ends the conversation: its life stops, so a turn in flight is called off, and the
// waits it held are released. It is the reader's explicit act — a new chat — or the
// bridge going inert, never a browser leaving the panel.
func (s *Session) End() int {
	if s.halt != nil {
		s.halt()
	}
	return s.Close()
}
