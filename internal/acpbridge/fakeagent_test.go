package acpbridge

import (
	"encoding/json"
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

// offeringAgent answers session/new with a `model` config option, the one place the
// agent's model list arrives, and accepts a set.
func offeringAgent(current string, choices ...string) handler {
	return acptest.OffersModels(current, choices...)
}

// setRefusingAgent offers a model list at session/new, refuses every set with the agent's own
// sentence — the shape a provider's refusal has — and still answers a turn, so a test can see
// that a refused declaration left a usable conversation behind.
func setRefusingAgent(message string) handler {
	models := acptest.OffersModels("default/model", "default/model")
	streams := acptest.Streams("the answer")
	return func(agent *fakeAgent, method string, params json.RawMessage) any {
		switch method {
		case "session/set_config_option":
			return rpcError{Code: -32602, Message: message}
		case "session/new":
			return models(agent, method, params)
		}
		return streams(agent, method, params)
	}
}
