package acpbridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

// modelRefusing builds an agent that offers a model list and refuses the named model at
// set time while accepting any other — the shape a provider's quota refusal has.
func modelRefusing(refuse ...string) handler {
	offers := acptest.OffersModels("default/model", "good/model")
	refused := make(map[string]bool, len(refuse))
	for _, m := range refuse {
		refused[m] = true
	}
	return func(agent *fakeAgent, method string, params json.RawMessage) any {
		if method == "session/set_config_option" {
			var request struct {
				Value string `json:"value"`
			}
			if err := json.Unmarshal(params, &request); err == nil && refused[request.Value] {
				return rpcError{Code: -32602, Message: "the provider has no quota for " + request.Value}
			}
		}
		return offers(agent, method, params)
	}
}

// TestDeclaredModelsAreTriedInOrder proves the fallback: the first model is refused, the
// second is accepted, and the session ends up on the accepted one with nothing to report.
func TestDeclaredModelsAreTriedInOrder(t *testing.T) {
	agent := newFakeAgent(t, modelRefusing("exhausted/model"))
	bridge := New(Options{
		Root:    t.TempDir(),
		Version: "test",
		Models:  []string{"exhausted/model", "good/model"},
	})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("a fallback must open the session: %v", err)
	}
	if got := bridge.Posture().Model; got != "good/model" {
		t.Fatalf("the posture names %q, want the accepted model", got)
	}
	if status, note := bridge.Status(); status != StatusActive || note != "" {
		t.Fatalf("status=%q note=%q, want an active session and no sentence", status, note)
	}
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	if got := modelInUse(t, session.State().Snapshot()); got != "good/model" {
		t.Fatalf("the session is on %q, want the accepted model", got)
	}
}

// TestAllDeclaredModelsRefusedLeaveOneSentence: the session still opens, the reader reads
// one sentence naming the list, and the posture keeps the first declared model so the
// panel does not show a value nobody named.
func TestAllDeclaredModelsRefusedLeaveOneSentence(t *testing.T) {
	agent := newFakeAgent(t, modelRefusing("first/model", "second/model"))
	bridge := New(Options{
		Root:    t.TempDir(),
		Version: "test",
		Models:  []string{"first/model", "second/model"},
	})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("a refused list must not fail the open: %v", err)
	}
	status, note := bridge.Status()
	if status != StatusActive {
		t.Fatalf("a refused list must leave the session live: status=%q", status)
	}
	if !strings.Contains(note, "any of the declared models") || !strings.Contains(note, "first/model") {
		t.Fatalf("the sentence must name the list, got %q", note)
	}
	if got := bridge.Posture().Model; got != "first/model" {
		t.Fatalf("the posture names %q, want the first declared model", got)
	}
}

// TestASingleDeclaredModelKeepsTheSingularSentence keeps the sentence a project with one
// model already reads: a list changed the mechanism, not the words.
func TestASingleDeclaredModelKeepsTheSingularSentence(t *testing.T) {
	agent := newFakeAgent(t, modelRefusing("only/model"))
	bridge := New(Options{Root: t.TempDir(), Version: "test", Models: []string{"only/model"}})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("a refused model must not fail the open: %v", err)
	}
	if _, note := bridge.Status(); !strings.Contains(note, `would not take the declared model "only/model"`) {
		t.Fatalf("the singular sentence changed: %q", note)
	}
}
