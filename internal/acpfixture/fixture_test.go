package acpfixture

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestServeAnswersTheHandshakeAndStreamsTheScript drives the fixture the way the bridge
// does: requests one per line, notifications and results back.
func TestServeAnswersTheHandshakeAndStreamsTheScript(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"session/new"}`,
		`{"jsonrpc":"2.0","id":3,"method":"session/prompt"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := Serve(strings.NewReader(in), &out, ScriptAnswer); err != nil {
		t.Fatalf("serve: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("output = %d lines, want the handshake and the stream", len(lines))
	}
	var chunks int
	var sawEnd bool
	for _, line := range lines {
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("line is not JSON: %q", line)
		}
		if msg["method"] == "session/update" {
			chunks++
		}
		if result, ok := msg["result"].(map[string]any); ok && result["stopReason"] == "end_turn" {
			sawEnd = true
		}
	}
	if chunks == 0 || !sawEnd {
		t.Fatalf("chunks=%d end=%v, want a streamed answer and an end_turn", chunks, sawEnd)
	}
}

func TestServeRefusesTheTurnOnTheFailScript(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":3,"method":"session/prompt"}` + "\n"
	var out bytes.Buffer
	if err := Serve(strings.NewReader(in), &out, ScriptFail); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(out.String(), "provider.free-limit") {
		t.Fatalf("the failure script must carry the provider sentence: %s", out.String())
	}
}
