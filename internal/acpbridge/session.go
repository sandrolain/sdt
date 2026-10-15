package acpbridge

import (
	"context"
	"fmt"

	"github.com/coder/acp-go-sdk"
)

// Session is one browser connection's conversation: the ACP session it opened and
// the thread it accumulates. One browser connection owns one session; the viewer
// opens a session when a browser arrives and closes it when the browser leaves.
type Session struct {
	id     string
	conn   *acp.ClientSideConnection
	state  *ThreadState
	perms  *PermissionQueue
	detach func()
}

// ID is the ACP session id the agent assigned.
func (s *Session) ID() string { return s.id }

// State is the thread this session accumulates.
func (s *Session) State() *ThreadState { return s.state }

// Prompt sends the user's text and returns when the turn ends. It blocks for the
// whole turn, so a caller that also serves the browser's cancel has to call it
// from its own goroutine — which is what the WebSocket surface does.
func (s *Session) Prompt(ctx context.Context, text string) error {
	id := s.state.openUserMessage(text)
	response, err := s.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(s.id),
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
	if err := s.conn.Cancel(ctx, acp.CancelNotification{SessionId: acp.SessionId(s.id)}); err != nil {
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
	cancelled := s.perms.CancelSession(s.id)
	if s.detach != nil {
		s.detach()
	}
	return cancelled
}
