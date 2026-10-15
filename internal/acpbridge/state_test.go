package acpbridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// The variant constructors, written by hand so each test states the wire shape it
// feeds the fold rather than relying on a helper's defaults.
func userChunk(text, id string) acp.SessionUpdate {
	chunk := &acp.SessionUpdateUserMessageChunk{Content: acp.TextBlock(text)}
	if id != "" {
		chunk.MessageId = &id
	}
	return acp.SessionUpdate{UserMessageChunk: chunk}
}

func agentChunk(text, id string) acp.SessionUpdate {
	chunk := &acp.SessionUpdateAgentMessageChunk{Content: acp.TextBlock(text)}
	if id != "" {
		chunk.MessageId = &id
	}
	return acp.SessionUpdate{AgentMessageChunk: chunk}
}

func thoughtChunk(text string) acp.SessionUpdate {
	return acp.SessionUpdate{AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{Content: acp.TextBlock(text)}}
}

func status(value string) *acp.ToolCallStatus {
	converted := acp.ToolCallStatus(value)
	return &converted
}

func apply(t *testing.T, state *ThreadState, updates ...acp.SessionUpdate) {
	t.Helper()
	for _, update := range updates {
		if err := state.SessionUpdate(context.Background(), acp.SessionNotification{
			SessionId: "sess-1",
			Update:    update,
		}); err != nil {
			t.Fatalf("SessionUpdate: %v", err)
		}
	}
}

// One scripted session, every variant the protocol defines, folded in the order an
// agent sends them. The assertions read the thread after each interesting variant,
// so a single test carries the whole mapping table without hiding a step.
func TestFoldScriptedSession(t *testing.T) {
	state := NewThreadState("sess-1")

	steps := []struct {
		name   string
		update acp.SessionUpdate
	}{
		{"the user's message arrives", userChunk("what changed?", "")},
		{"the agent starts talking", agentChunk("Let me look", "")},
		{"a chunk of the same message", agentChunk(" at the notes.", "")},
		{"the agent reasons", thoughtChunk("checking the corpus")},
		{"a tool call starts", acp.SessionUpdate{ToolCall: &acp.SessionUpdateToolCall{
			ToolCallId: "call-1",
			Title:      "Read context/notes/a.md",
			Kind:       acp.ToolKind("read"),
			Status:     acp.ToolCallStatus("in_progress"),
			RawInput:   map[string]any{"path": "context/notes/a.md"},
		}}},
		{"the tool call finishes", acp.SessionUpdate{ToolCallUpdate: &acp.SessionToolCallUpdate{
			ToolCallId: "call-1",
			Status:     status("completed"),
			RawOutput:  "the file's text",
		}}},
		{"the agent plans", acp.SessionUpdate{Plan: &acp.SessionUpdatePlan{
			Entries: []acp.PlanEntry{
				{Content: "read the notes", Priority: "medium", Status: "pending"},
				{Content: "answer", Priority: "high", Status: "pending"},
			},
		}}},
		{"the plan is updated by id", acp.SessionUpdate{PlanUpdate: &acp.SessionPlanUpdate{
			Plan: acp.NewPlanUpdateContentItems("plan-1", []acp.PlanEntry{
				{Content: "read the notes", Priority: "medium", Status: "completed"},
			}),
		}}},
		{"the agent offers commands", acp.SessionUpdate{AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
			AvailableCommands: []acp.AvailableCommand{{Name: "/compact", Description: "Compact the session"}},
		}}},
		{"the mode changes", acp.SessionUpdate{CurrentModeUpdate: &acp.SessionCurrentModeUpdate{
			CurrentModeId: acp.SessionModeId("plan"),
		}}},
		{"the config changes", acp.SessionUpdate{ConfigOptionUpdate: &acp.SessionConfigOptionUpdate{
			ConfigOptions: []acp.SessionConfigOption{acp.NewSessionConfigOptionBoolean(true)},
		}}},
		{"the session is renamed", acp.SessionUpdate{SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			Title: acp.Ptr("Reading the corpus"),
		}}},
		{"the usage is reported", acp.SessionUpdate{UsageUpdate: &acp.SessionUsageUpdate{
			Used: 1200,
			Size: 8000,
			Cost: &acp.Cost{Amount: 0.02, Currency: "USD"},
		}}},
	}

	for _, step := range steps {
		apply(t, state, step.update)
		snapshot := state.Snapshot()
		switch step.name {
		case "a chunk of the same message":
			// Chunks append to the same part: the agent streams one message.
			parts := snapshot.Messages[1].Parts
			if len(parts) != 1 || parts[0].Text != "Let me look at the notes." {
				t.Fatalf("streamed text = %+v, want one part with both chunks", parts)
			}
		case "the tool call finishes":
			call := snapshot.Messages[1].Parts[2].ToolCall
			if call == nil || call.Status != "completed" || call.Output != "the file's text" {
				t.Fatalf("the finished tool call = %+v", call)
			}
		case "the agent plans":
			if snapshot.Plan == nil || len(snapshot.Plan.Entries) != 2 {
				t.Fatalf("the plan = %+v, want two entries", snapshot.Plan)
			}
		case "the plan is updated by id":
			if snapshot.Plan == nil || snapshot.Plan.ID != "plan-1" || len(snapshot.Plan.Entries) != 1 {
				t.Fatalf("the updated plan = %+v", snapshot.Plan)
			}
		case "the session is renamed":
			if snapshot.Info == nil || snapshot.Info.Title != "Reading the corpus" {
				t.Fatalf("the session info = %+v", snapshot.Info)
			}
		}
	}

	// The plan is removed by the unstable variant, and the thread keeps everything
	// else: a removal is an update, not a reset.
	apply(t, state, acp.SessionUpdate{PlanRemoved: &acp.SessionUpdatePlanRemoved{Id: "plan-1"}})
	final := state.Snapshot()
	if final.Plan != nil {
		t.Fatalf("plan_removed left %+v", final.Plan)
	}
	assertFoldedThread(t, final)
}

// assertFoldedThread checks the thread the scripted session ends with. It is a
// helper rather than more cases in the test so the test reads as the sequence it
// is.
func assertFoldedThread(t *testing.T, final Thread) {
	t.Helper()
	if len(final.Messages) != 2 {
		t.Fatalf("the thread holds %d messages, want the user's and the agent's", len(final.Messages))
	}
	if final.Messages[0].Role != RoleUser || final.Messages[0].Parts[0].Text != "what changed?" {
		t.Fatalf("the user's message = %+v", final.Messages[0])
	}
	kinds := make([]string, 0, len(final.Messages[1].Parts))
	for _, part := range final.Messages[1].Parts {
		kinds = append(kinds, part.Kind)
	}
	want := []string{PartText, PartThought, PartToolCall}
	if len(kinds) != len(want) {
		t.Fatalf("the agent's parts = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("the agent's parts = %v, want %v", kinds, want)
		}
	}
	if final.Mode != "plan" {
		t.Fatalf("the mode = %q", final.Mode)
	}
	if len(final.Commands) != 1 || final.Commands[0].Name != "/compact" {
		t.Fatalf("the commands = %+v", final.Commands)
	}
	if len(final.Config) != 1 {
		t.Fatalf("the config = %+v", final.Config)
	}
	if final.Usage == nil || final.Usage.Used != 1200 || final.Usage.Size != 8000 || final.Usage.Amount != 0.02 {
		t.Fatalf("the usage = %+v", final.Usage)
	}
}

// The case the plan names: a chunk that arrives after a tool_call_update is new
// text after the tool call, not more text before it.
func TestAChunkAfterAToolCallUpdateOpensANewPart(t *testing.T) {
	state := NewThreadState("sess-1")
	apply(t, state,
		acp.SessionUpdate{ToolCall: &acp.SessionUpdateToolCall{ToolCallId: "call-1", Title: "Read a file"}},
		acp.SessionUpdate{ToolCallUpdate: &acp.SessionToolCallUpdate{ToolCallId: "call-1", Status: status("completed")}},
		agentChunk("The file says this.", ""),
	)

	parts := state.Snapshot().Messages[0].Parts
	if len(parts) != 2 {
		t.Fatalf("the assistant turn holds %d parts, want the tool call and the text: %+v", len(parts), parts)
	}
	if parts[0].Kind != PartToolCall || parts[1].Kind != PartText || parts[1].Text != "The file says this." {
		t.Fatalf("the parts are %+v", parts)
	}
}

// The user's own text is visible the moment the prompt goes out, and the agent's
// echo of it replaces that text instead of doubling it.
func TestTheProvisionalUserMessageReconcilesWithTheEcho(t *testing.T) {
	state := NewThreadState("sess-1")
	id := state.openUserMessage("what changed?")

	withID := state.Snapshot().Messages[0]
	if !withID.Provisional || withID.Parts[0].Text != "what changed?" {
		t.Fatalf("the provisional message = %+v", withID)
	}

	apply(t, state, userChunk("what changed?", id))
	echoed := state.Snapshot().Messages[0]
	if echoed.Provisional {
		t.Fatal("the echo left the message provisional")
	}
	if len(echoed.Parts) != 1 || echoed.Parts[0].Text != "what changed?" {
		t.Fatalf("the echo produced %+v, want the same one part", echoed.Parts)
	}

	// A further chunk of the same message appends, as a stream does.
	apply(t, state, userChunk(" and why?", id))
	final := state.Snapshot().Messages[0]
	if len(final.Parts) != 1 || final.Parts[0].Text != "what changed? and why?" {
		t.Fatalf("the appended text = %+v", final.Parts)
	}
}

// A permission card is a part of the agent's turn, and the answer resolves it
// where it stands: the position and the content do not move.
func TestAPermissionCardResolvesInPlace(t *testing.T) {
	state := NewThreadState("sess-1")
	ctx := context.Background()
	state.Permission(ctx, Permission{
		RequestID:  "perm-1",
		SessionID:  "sess-1",
		ToolCallID: "call-1",
		ToolKind:   "edit",
		Title:      "Write context/notes/x.md",
		Input:      map[string]any{"path": "context/notes/x.md"},
	})

	before := state.Snapshot()
	if len(before.Messages) != 1 || len(before.Messages[0].Parts) != 1 {
		t.Fatalf("the card did not open an assistant turn: %+v", before.Messages)
	}
	card := before.Messages[0].Parts[0]
	if card.Kind != PartPermission || card.Permission == nil || card.Permission.ToolKind != "edit" {
		t.Fatalf("the card = %+v", card)
	}
	if card.Permission.Decision != "" {
		t.Fatalf("a fresh card already carries a decision: %q", card.Permission.Decision)
	}

	if !state.ResolvePermission("perm-1", DecisionAllowed) {
		t.Fatal("the card was not found by its request id")
	}
	after := state.Snapshot()
	if len(after.Messages[0].Parts) != 1 {
		t.Fatalf("resolving the card moved the thread: %+v", after.Messages[0].Parts)
	}
	resolved := after.Messages[0].Parts[0].Permission
	if resolved.Decision != DecisionAllowed || resolved.Title != card.Permission.Title || resolved.ToolKind != "edit" {
		t.Fatalf("the resolved card = %+v", resolved)
	}
	if state.ResolvePermission("perm-not-a-request", DecisionDenied) {
		t.Fatal("an unknown request id was reported as resolved")
	}
}

// An update for a call this client never saw still lands: the protocol makes the
// tool call's id its identity.
func TestAToolCallUpdateForAnUnseenCallOpensThePart(t *testing.T) {
	state := NewThreadState("sess-1")
	apply(t, state, acp.SessionUpdate{ToolCallUpdate: &acp.SessionToolCallUpdate{
		ToolCallId: "call-9",
		Status:     status("completed"),
	}})

	parts := state.Snapshot().Messages[0].Parts
	if len(parts) != 1 || parts[0].ToolCall == nil || parts[0].ToolCall.ID != "call-9" {
		t.Fatalf("the parts = %+v", parts)
	}
}

// A variant this build does not model is dropped, never fatal — the frontend is
// one protocol version behind the agent by design.
func TestAnUnknownVariantIsIgnored(t *testing.T) {
	state := NewThreadState("sess-1")
	apply(t, state, agentChunk("before", ""))
	before := state.Snapshot()

	// Every pointer nil is what the SDK produces when it matched no variant.
	apply(t, state, acp.SessionUpdate{})
	after := state.Snapshot()
	if len(after.Messages) != len(before.Messages) || after.Messages[0].Parts[0].Text != "before" {
		t.Fatalf("an empty variant changed the thread: %+v", after.Messages)
	}

	// What the SDK does with an unknown *wire* discriminator is the SDK's business —
	// it may classify the body by shape or refuse it. The fold's contract is only
	// that whatever reaches it is applied or dropped without failing.
	var wire acp.SessionUpdate
	if err := json.Unmarshal([]byte(`{"sessionUpdate":"a_future_thing","content":{"type":"text","text":"hi"}}`), &wire); err == nil {
		apply(t, state, wire)
	}
}

// A listener is fed on every applied update — that is how a browser connection is
// written to — and a snapshot does not move under its reader.
func TestAListenerSeesEveryStateAndSnapshotsAreIndependent(t *testing.T) {
	state := NewThreadState("sess-1")
	states := make(chan Thread, 8)
	state.Listen(func(thread Thread) { states <- thread })

	apply(t, state, agentChunk("first", ""))
	first := <-states
	apply(t, state, agentChunk(" second", ""))

	if first.Messages[0].Parts[0].Text != "first" {
		t.Fatalf("a snapshot changed after it was taken: %q", first.Messages[0].Parts[0].Text)
	}
	second := <-states
	if second.Messages[0].Parts[0].Text != "first second" {
		t.Fatalf("the second state = %q", second.Messages[0].Parts[0].Text)
	}
}
