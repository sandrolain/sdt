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
	// EnvelopeConfig sets one configuration option of a session — the model it runs
	// on, its mode. It is general in the protocol's own terms; the panel offers a
	// control for the model and nothing else.
	EnvelopeConfig = "config"
	// EnvelopeStatus is the bridge's posture, sent when it changes rather than
	// when a browser asks: a restart that begins and ends between two reads is
	// still a restart the reader has to see.
	EnvelopeStatus = "status"
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

// ConfigEnvelope sets one configuration option of a session. The option is named by
// its own id and set to one of the values the agent offered, both verbatim: the
// viewer never maps a model name, so what the agent advertised is what it is asked
// for.
type ConfigEnvelope struct {
	Type      string `json:"type"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
	SessionID string `json:"sessionId,omitempty"`
}

// Command is one message from the browser, decoded from its envelope.
type Command struct {
	Kind      string
	Text      string
	RequestID string
	Allow     bool
	ConfigID  string
	Value     string
	SessionID string
}

// EncodeState marshals the state envelope for a thread.
func EncodeState(sessionID string, thread Thread) ([]byte, error) {
	return json.Marshal(StateEnvelope{Type: EnvelopeState, SessionID: sessionID, Thread: thread})
}

// StatusEnvelope is the posture of the agent behind one stream: the fields the
// state endpoint reports, minus the thread (a `state` frame is the thread) and
// minus `command` (it does not change while a stream is open, and the panel
// reads it once when it opens).
type StatusEnvelope struct {
	Type      string `json:"type"`
	Activated bool   `json:"activated"`
	Connected bool   `json:"connected"`
	Status    string `json:"status"`
	SessionID string `json:"sessionId,omitempty"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"`
}

// EncodeStatus serialises one posture frame. The viewer builds it; the bridge
// never knows about HTTP.
func EncodeStatus(envelope StatusEnvelope) ([]byte, error) {
	envelope.Type = EnvelopeStatus
	return json.Marshal(envelope)
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
	case EnvelopeConfig:
		var config ConfigEnvelope
		if err := json.Unmarshal(data, &config); err != nil {
			return Command{}, fmt.Errorf("%w: config: %v", ErrCommand, err)
		}
		if config.ConfigID == "" {
			return Command{}, fmt.Errorf("%w: a config command names no option", ErrCommand)
		}
		if config.Value == "" {
			return Command{}, fmt.Errorf("%w: a config command carries no value", ErrCommand)
		}
		return Command{
			Kind:      EnvelopeConfig,
			ConfigID:  config.ConfigID,
			Value:     config.Value,
			SessionID: config.SessionID,
		}, nil
	default:
		return Command{}, fmt.Errorf("%w: %q", ErrCommand, envelope.Type)
	}
}
