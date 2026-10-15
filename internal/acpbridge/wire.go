package acpbridge

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The wire between the viewer and the browser: one JSON envelope per event, and
// the whole thread state per notification, so the browser never reassembles
// chunks. The names below are the single source of them — one TS module mirrors
// them and the golden fixtures of phase 6 keep the two from drifting.

// Envelope kinds.
const (
	// EnvelopeState is what the viewer sends: the whole thread of one session.
	EnvelopeState = "state"
	// EnvelopePrompt is a user message.
	EnvelopePrompt = "prompt"
	// EnvelopeCancel stops the current turn.
	EnvelopeCancel = "cancel"
	// EnvelopePermission answers a pending request.
	EnvelopePermission = "permission"
)

// ErrCommand is returned for a browser message the wire does not define. The
// socket carries a closed vocabulary, so an unknown type is a refusal rather than
// something to guess at.
var ErrCommand = errors.New("acpbridge: unknown command from the browser")

// StateEnvelope is the whole thread of one session.
type StateEnvelope struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Thread    Thread `json:"thread"`
}

// PromptEnvelope carries the user's text.
type PromptEnvelope struct {
	Type string `json:"type"`
	Text string `json:"text"`
	// SessionID names the conversation, because a command travels as its own HTTP
	// request while the stream is a different one. Empty means "the stream I am
	// reading", which is the single-browser case.
	SessionID string `json:"sessionId,omitempty"`
}

// CancelEnvelope asks the agent to stop the current turn.
type CancelEnvelope struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
}

// PermissionEnvelope is the user's answer to a pending request.
type PermissionEnvelope struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	Allow     bool   `json:"allow"`
	SessionID string `json:"sessionId,omitempty"`
}

// Command is one message from the browser, decoded from its envelope.
type Command struct {
	Kind      string
	Text      string
	RequestID string
	Allow     bool
	SessionID string
}

// EncodeState marshals the state envelope for a thread.
func EncodeState(sessionID string, thread Thread) ([]byte, error) {
	return json.Marshal(StateEnvelope{Type: EnvelopeState, SessionID: sessionID, Thread: thread})
}

// DecodeCommand parses one browser message. Every field the envelope carries is
// required by the protocol this viewer speaks, so a malformed one is refused with
// ErrCommand rather than half-applied.
func DecodeCommand(data []byte) (Command, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Command{}, fmt.Errorf("%w: %v", ErrCommand, err)
	}
	switch envelope.Type {
	case EnvelopePrompt:
		var prompt PromptEnvelope
		if err := json.Unmarshal(data, &prompt); err != nil {
			return Command{}, fmt.Errorf("%w: prompt: %v", ErrCommand, err)
		}
		if prompt.Text == "" {
			return Command{}, fmt.Errorf("%w: a prompt carries no text", ErrCommand)
		}
		return Command{Kind: EnvelopePrompt, Text: prompt.Text, SessionID: prompt.SessionID}, nil
	case EnvelopeCancel:
		var cancel CancelEnvelope
		if err := json.Unmarshal(data, &cancel); err != nil {
			return Command{}, fmt.Errorf("%w: cancel: %v", ErrCommand, err)
		}
		return Command{Kind: EnvelopeCancel, SessionID: cancel.SessionID}, nil
	case EnvelopePermission:
		var permission PermissionEnvelope
		if err := json.Unmarshal(data, &permission); err != nil {
			return Command{}, fmt.Errorf("%w: permission: %v", ErrCommand, err)
		}
		if permission.RequestID == "" {
			return Command{}, fmt.Errorf("%w: an answer names no request", ErrCommand)
		}
		return Command{
			Kind:      EnvelopePermission,
			RequestID: permission.RequestID,
			Allow:     permission.Allow,
			SessionID: permission.SessionID,
		}, nil
	default:
		return Command{}, fmt.Errorf("%w: %q", ErrCommand, envelope.Type)
	}
}
