package acpbridge

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// The command vocabulary the browser speaks, decoded: the viewer refuses anything
// outside it, so the shapes here are the contract, not a convenience.
func TestDecodeCommandAcceptsTheBrowserVocabulary(t *testing.T) {
	cases := []struct {
		name string
		body string
		want Command
	}{
		{
			"a prompt",
			`{"type":"prompt","text":"how big is the corpus?"}`,
			Command{Kind: EnvelopePrompt, Text: "how big is the corpus?"},
		},
		{
			"a prompt naming its session",
			`{"type":"prompt","text":"and the plans?","sessionId":"sess-2"}`,
			Command{Kind: EnvelopePrompt, Text: "and the plans?", SessionID: "sess-2"},
		},
		{"a cancel", `{"type":"cancel"}`, Command{Kind: EnvelopeCancel}},
		{
			"a permission answer",
			`{"type":"permission","requestId":"perm-1","allow":true}`,
			Command{Kind: EnvelopePermission, RequestID: "perm-1", Allow: true},
		},
		{
			"a permission answer naming its session",
			`{"type":"permission","requestId":"perm-2","allow":false,"sessionId":"sess-3"}`,
			Command{Kind: EnvelopePermission, RequestID: "perm-2", Allow: false, SessionID: "sess-3"},
		},
		{
			"a configuration option set",
			`{"type":"config","configId":"model","value":"opencode/big-pickle"}`,
			Command{Kind: EnvelopeConfig, ConfigID: "model", Value: "opencode/big-pickle"},
		},
		{
			"a configuration option set on a named session",
			`{"type":"config","configId":"model","value":"openrouter/meta/muse-spark-open","sessionId":"sess-2"}`,
			Command{Kind: EnvelopeConfig, ConfigID: "model", Value: "openrouter/meta/muse-spark-open", SessionID: "sess-2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeCommand([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeCommand(%s) = %v", tc.body, err)
			}
			if got != tc.want {
				t.Fatalf("DecodeCommand(%s) = %+v, want %+v", tc.body, got, tc.want)
			}
		})
	}
}

// Everything the wire does not define is refused with ErrCommand rather than half
// applied: an unknown type, a prompt with no text, an answer with no request.
func TestDecodeCommandRefusesWhatTheWireDoesNotDefine(t *testing.T) {
	bodies := []string{
		`{"type":"something-else"}`,
		`{"type":"prompt"}`,
		`{"type":"prompt","text":""}`,
		`{"type":"permission","allow":true}`,
		`{"type":"cancel","sessionId":42}`,
		`{"type":"config","value":"opencode/big-pickle"}`,
		`{"type":"config","configId":"model"}`,
		`{"type":"config","configId":"","value":"opencode/big-pickle"}`,
		`{"type":"config","configId":"model","value":""}`,
		`not json at all`,
		``,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			if _, err := DecodeCommand([]byte(body)); !errors.Is(err, ErrCommand) {
				t.Fatalf("DecodeCommand(%q) = %v, want ErrCommand", body, err)
			}
		})
	}
}

// The state envelope is what the browser renders from: the whole thread, once, under
// the kind the client module knows.
func TestEncodeStateCarriesTheWholeThread(t *testing.T) {
	thread := NewThreadState("sess-1")
	update := acp.SessionNotification{SessionId: "sess-1", Update: agentChunk("hello", "")}
	if err := thread.SessionUpdate(t.Context(), update); err != nil {
		t.Fatalf("SessionUpdate: %v", err)
	}
	payload, err := EncodeState("sess-1", thread.Snapshot())
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	var envelope StateEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Type != EnvelopeState || envelope.SessionID != "sess-1" {
		t.Fatalf("the envelope is %+v", envelope)
	}
	if len(envelope.Thread.Messages) != 1 || envelope.Thread.Messages[0].Parts[0].Text != "hello" {
		t.Fatalf("the thread is %+v", envelope.Thread)
	}
}
