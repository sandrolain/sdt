package acpbridge

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// The dial policy is the wave's security boundary: a refused target is the
// difference between "the viewer talks to the agent the user named" and "the
// viewer talks to whatever a path happens to point at". Every refusal is asserted
// here, and the accepted case sits next to them so the list cannot pass by
// refusing everything.
func TestCheckDialTargetRefusesWhatThePolicyDoesNotName(t *testing.T) {
	listener, err := net.Listen("unix", socketPath(t))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	accepted := listener.Addr().String()

	plain := filepath.Join(t.TempDir(), "not-a-socket.txt")
	if err := os.WriteFile(plain, []byte("x"), 0o600); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"relative", "agent.sock"},
		{"host and port", "127.0.0.1:8791"},
		{"a URL", "unix:///tmp/sdt-agent.sock"},
		{"over the kernel limit", "/" + strings.Repeat("x", maxSocketPath)},
		{"missing", filepath.Join(t.TempDir(), "absent.sock")},
		{"not a socket", plain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkDialTarget(tc.path); !errors.Is(err, ErrDialTarget) {
				t.Fatalf("checkDialTarget(%q) = %v, want ErrDialTarget", tc.path, err)
			}
			// The policy runs before the dial, so a refused target never opens a
			// connection at all.
			if _, err := dialSocket(context.Background(), tc.path); !errors.Is(err, ErrDialTarget) {
				t.Fatalf("dialSocket(%q) = %v, want ErrDialTarget", tc.path, err)
			}
		})
	}

	if err := checkDialTarget(accepted); err != nil {
		t.Fatalf("checkDialTarget(%q) = %v, want nil", accepted, err)
	}
}

// A socket that exists but answers nothing is "the agent is not reachable", a
// different state for the user than a refused target: one is a wrong path, the
// other is an agent that is not running.
func TestDialSocketReportsAnUnreachableAgent(t *testing.T) {
	if _, err := dialSocket(context.Background(), staleSocketPath(t)); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("dialSocket(stale) = %v, want ErrUnreachable", err)
	}
}

// The bridge is inert by construction: it dials nothing until Activate runs, and
// the fake agent is the witness.
func TestBridgeIsInertBeforeActivation(t *testing.T) {
	agent := newFakeAgent(t, agentVersion(acp.ProtocolVersionNumber, "sess-1"))
	bridge := New(Options{Root: t.TempDir(), Version: "test"})

	if bridge.Activated() {
		t.Fatal("a fresh bridge reports itself activated")
	}
	if got := bridge.SessionID(); got != "" {
		t.Fatalf("SessionID() = %q, want empty", got)
	}
	if got := agent.connections(); got != 0 {
		t.Fatalf("the agent accepted %d connections before activation", got)
	}
}
