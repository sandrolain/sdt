package acpbridge

import (
	"testing"

	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

// The tests drive the shared ACP peer (internal/acpbridge/acptest) through the
// names they read: the double serves the viewer's tests as well, so it lives in a
// package of its own and these aliases keep the bodies focused on the subject.
type (
	fakeAgent = acptest.Agent
	handler   = acptest.Handler
	rawReply  = acptest.RawReply
	rpcError  = acptest.RPCError
)

func newFakeAgent(t *testing.T, h handler) *fakeAgent { return acptest.NewAgent(t, h) }

func socketPath(t *testing.T) string      { return acptest.SocketPath(t) }
func staleSocketPath(t *testing.T) string { return acptest.StaleSocketPath(t) }

func agentVersion(version int, sessionID string) handler {
	return acptest.AgentVersion(version, sessionID)
}

// streamingAgent answers a turn by streaming one chunk, the shape a real agent has.
func streamingAgent(text string) handler { return acptest.Streams(text) }

// refusingAgent refuses the turn the way a provider's refusal arrives: a JSON-RPC error.
func refusingAgent(code int, message string) handler { return acptest.Refuses(code, message) }

// stoppingAgent answers a turn with one chunk and the reason it stopped on.
func stoppingAgent(text, reason string) handler { return acptest.Stops(text, reason) }
