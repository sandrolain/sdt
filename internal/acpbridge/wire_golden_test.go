package acpbridge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// The wire fixtures are the contract between the two halves of the wave, written by
// the side that owns the wire and read by the side that renders it:
//
//   - `variants/<name>.json` — one state envelope per `session/update` variant, so the
//     browser's client module is exercised against every shape the Go fold can emit.
//   - `session.json` — a whole scripted session, the shape the panel actually sees.
//   - `commands/<name>.json` — the three envelopes the browser sends, which the Go
//     decoder accepts; the TS test asserts its own builders produce exactly these.
//
// Regenerate with `UPDATE_GOLDEN=1 go test ./internal/acpbridge/`, so a change to the
// wire is a deliberate act rather than something a refactor can do quietly.

// wireVariants is every variant with the shape only that variant produces. The order
// is the analysis's mapping table.
func wireVariants(t *testing.T) map[string]acp.SessionUpdate {
	t.Helper()
	return map[string]acp.SessionUpdate{
		"user_message_chunk":  userChunk("the user's words", "msg-user-1"),
		"agent_message_chunk": agentChunk("the agent's answer", "msg-agent-1"),
		"agent_thought_chunk": thoughtChunk("the agent's reasoning"),
		"tool_call":           {ToolCall: &acp.SessionUpdateToolCall{ToolCallId: "call-1", Title: "Read context/notes/a.md", Kind: acp.ToolKind("read"), Status: acp.ToolCallStatus("in_progress"), RawInput: map[string]any{"path": "context/notes/a.md"}}},
		"tool_call_update":    {ToolCallUpdate: &acp.SessionToolCallUpdate{ToolCallId: "call-1", Status: status("completed"), RawOutput: "the file's text"}},
		"plan":                {Plan: &acp.SessionUpdatePlan{Entries: []acp.PlanEntry{{Content: "read the notes", Priority: "medium", Status: "pending"}}}},
		"plan_update":         {PlanUpdate: &acp.SessionPlanUpdate{Plan: acp.NewPlanUpdateContentItems("plan-1", []acp.PlanEntry{{Content: "read the notes", Priority: "medium", Status: "completed"}})}},
		"plan_removed":        {PlanRemoved: &acp.SessionUpdatePlanRemoved{Id: "plan-1"}},
		"available_commands":  {AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{AvailableCommands: []acp.AvailableCommand{{Name: "/compact", Description: "Compact the session"}}}},
		"current_mode":        {CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: acp.SessionModeId("plan")}},
		"config_option":       {ConfigOptionUpdate: &acp.SessionConfigOptionUpdate{ConfigOptions: []acp.SessionConfigOption{acp.NewSessionConfigOptionBoolean(true)}}},
		"session_info":        {SessionInfoUpdate: &acp.SessionSessionInfoUpdate{Title: acp.Ptr("Reading the corpus")}},
		"usage":               {UsageUpdate: &acp.SessionUsageUpdate{Used: 1200, Size: 8000, Cost: &acp.Cost{Amount: 0.02, Currency: "USD"}}},
		"permission":          {},
	}
}

// wireSession is the scripted session `session.json` carries: the same sequence the
// fold's own test drives, so the fixture and the unit test cannot say different things.
func wireSession(t *testing.T) *ThreadState {
	t.Helper()
	state := NewThreadState("sess-1")
	apply(t, state,
		userChunk("how big is the corpus?", ""),
		agentChunk("Let me look", ""),
		agentChunk(" at the notes.", ""),
		thoughtChunk("checking the corpus"),
		acp.SessionUpdate{ToolCall: &acp.SessionUpdateToolCall{ToolCallId: "call-1", Title: "Read context/notes/a.md", Kind: acp.ToolKind("read"), Status: acp.ToolCallStatus("in_progress")}},
		acp.SessionUpdate{ToolCallUpdate: &acp.SessionToolCallUpdate{ToolCallId: "call-1", Status: status("completed"), RawOutput: "224 analyses"}},
		agentChunk(" The corpus has 224 analyses.", ""),
		acp.SessionUpdate{Plan: &acp.SessionUpdatePlan{Entries: []acp.PlanEntry{{Content: "read the notes", Priority: "medium", Status: "completed"}}}},
	)
	// A permission card, resolved in place: the pair is what the panel renders.
	state.Permission(t.Context(), Permission{
		RequestID:  "perm-1",
		SessionID:  "sess-1",
		ToolCallID: "call-2",
		ToolKind:   "edit",
		Title:      "Write context/notes/x.md",
		Input:      map[string]any{"path": "context/notes/x.md"},
	})
	state.ResolvePermission("perm-1", DecisionAllowed)
	return state
}

func TestWireFixturesMatchTheBridge(t *testing.T) {
	dir := filepath.Join("testdata", "wire")
	fixtures, err := wireFixtureBytes(t)
	if err != nil {
		t.Fatalf("build fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures were built")
	}
	regenerate := os.Getenv("UPDATE_GOLDEN") == "1"
	for name, payload := range fixtures {
		path := filepath.Join(dir, name)
		if regenerate {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
			}
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			continue
		}
		committed, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v (regenerate with UPDATE_GOLDEN=1)", path, err)
		}
		if !bytes.Equal(committed, payload) {
			t.Errorf("%s differs from what the bridge produces; regenerate with UPDATE_GOLDEN=1", path)
		}
	}
	if regenerate {
		t.Logf("wrote %d wire fixtures under %s", len(fixtures), dir)
	}
}

// indentedState encodes a thread as the browser receives it, indented so a change to
// the wire shows up as a reviewable diff rather than one long line.
func indentedState(t *testing.T, state *ThreadState) []byte {
	t.Helper()
	payload, err := json.MarshalIndent(StateEnvelope{
		Type:      EnvelopeState,
		SessionID: state.Snapshot().SessionID,
		Thread:    state.Snapshot(),
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	return append(payload, '\n')
}

// wireFixtureBytes builds every fixture in memory, so the comparison and the
// regeneration read the same bytes.
func wireFixtureBytes(t *testing.T) (map[string][]byte, error) {
	t.Helper()
	fixtures := map[string][]byte{}

	// One file per variant, applied alone so its shape is the only thing in the frame.
	for name, update := range wireVariants(t) {
		if name == "permission" {
			// The permission card is not a session/update: it is the bridge asking the
			// user, so it is built the way the bridge builds it.
			state := NewThreadState("sess-1")
			state.Permission(t.Context(), Permission{
				RequestID: "perm-1",
				SessionID: "sess-1",
				ToolKind:  "edit",
				Title:     "Write context/notes/x.md",
				Input:     map[string]any{"path": "context/notes/x.md"},
			})
			fixtures[filepath.Join("variants", name+".json")] = indentedState(t, state)
			continue
		}
		state := NewThreadState("sess-1")
		if err := state.SessionUpdate(t.Context(), acp.SessionNotification{SessionId: "sess-1", Update: update}); err != nil {
			return nil, err
		}
		fixtures[filepath.Join("variants", name+".json")] = indentedState(t, state)
	}

	fixtures["session.json"] = indentedState(t, wireSession(t))

	// The three envelopes the browser sends. The TS test asserts its builders produce
	// exactly these, which is what makes a rename on either side a failure.
	commands := map[string]any{
		"prompt":     PromptEnvelope{Type: EnvelopePrompt, Text: "how big is the corpus?", SessionID: "sess-1"},
		"cancel":     CancelEnvelope{Type: EnvelopeCancel, SessionID: "sess-1"},
		"permission": PermissionEnvelope{Type: EnvelopePermission, RequestID: "perm-1", Allow: true, SessionID: "sess-1"},
	}
	for name, envelope := range commands {
		payload, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return nil, err
		}
		fixtures[filepath.Join("commands", name+".json")] = append(payload, '\n')
	}
	return fixtures, nil
}
