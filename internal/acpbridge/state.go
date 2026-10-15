package acpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Part kinds. Each names the session/update variant that produced it, so the
// browser's renderer and the protocol stay checkable against each other.
const (
	PartText       = "text"       // user_message_chunk, agent_message_chunk
	PartThought    = "thought"    // agent_thought_chunk
	PartToolCall   = "tool_call"  // tool_call, tool_call_update
	PartPermission = "permission" // a RequestPermission waiting for the user
)

// The user's answer to a permission card.
const (
	DecisionAllowed = "allowed"
	DecisionDenied  = "denied"
)

// Thread is the accumulated state of one ACP session: what the browser renders,
// and nothing else. The plan is thread-level rather than a part of a message,
// because a plan spans messages and is updated in place by id.
type Thread struct {
	SessionID string            `json:"sessionId"`
	Messages  []Message         `json:"messages"`
	Plan      *Plan             `json:"plan,omitempty"`
	Commands  []SlashCommand    `json:"commands,omitempty"`
	Mode      string            `json:"mode,omitempty"`
	Config    []json.RawMessage `json:"config,omitempty"`
	Info      *SessionInfo      `json:"info,omitempty"`
	Usage     *Usage            `json:"usage,omitempty"`
}

// Message is one user or assistant turn, with its parts in arrival order.
type Message struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	// Provisional marks the user's own message, written the moment the prompt is
	// sent: the agent's echo of it is what normally carries that text, and an agent
	// that never echoes would otherwise leave the user's own words invisible. The
	// first echo replaces the provisional parts and the flag clears, so the text is
	// never doubled.
	Provisional bool   `json:"-"`
	Parts       []Part `json:"parts"`
}

// Part is one piece of a message.
type Part struct {
	Kind       string      `json:"kind"`
	Text       string      `json:"text,omitempty"`
	ToolCall   *ToolCall   `json:"toolCall,omitempty"`
	Permission *Permission `json:"permission,omitempty"`
}

// ToolCall is one tool the agent ran, as the protocol reports it.
type ToolCall struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status,omitempty"`
	Input  any    `json:"input,omitempty"`
	Output any    `json:"output,omitempty"`
}

// Plan is the agent's execution plan. The unstable `plan_update` variant may carry
// a file or a markdown body rather than entries, and that payload is kept verbatim
// under Raw so nothing the agent sent is lost.
type Plan struct {
	ID      string          `json:"id,omitempty"`
	Entries []PlanEntry     `json:"entries,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

// PlanEntry is one step of a plan.
type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

// SlashCommand is one command the agent offers.
type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SessionInfo is the session's own metadata.
type SessionInfo struct {
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Usage is what the session has spent: the context window and, when the agent
// reports one, the cost.
type Usage struct {
	Used     int     `json:"used"`
	Size     int     `json:"size"`
	Amount   float64 `json:"amount,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

// ThreadState is one session's thread plus the order between the agent's updates
// and the browser's reads. The Go side is the single accumulator: the browser
// receives the whole thread and never reassembles chunks.
type ThreadState struct {
	mu       sync.Mutex
	thread   Thread
	listener func(Thread)
}

// NewThreadState returns the empty thread of one session.
func NewThreadState(sessionID string) *ThreadState {
	return &ThreadState{thread: Thread{SessionID: sessionID}}
}

// Listen registers the function called with a snapshot after each applied update.
// It is how a browser connection is fed; the last registration wins, because one
// connection renders one thread.
func (s *ThreadState) Listen(fn func(Thread)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listener = fn
}

// Snapshot returns a copy the caller may marshal without holding the lock.
func (s *ThreadState) Snapshot() Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneThread(s.thread)
}

// SessionUpdate folds one update into the thread and hands the result to the
// listener. The update is applied under the lock so the snapshot a listener
// receives is never a half-applied state.
func (s *ThreadState) SessionUpdate(_ context.Context, notification acp.SessionNotification) error {
	s.mu.Lock()
	s.apply(notification.Update)
	s.mu.Unlock()
	s.notify()
	return nil
}

// Permission adds the card to the turn the agent is streaming, so the user answers
// it where the tool call appears.
func (s *ThreadState) Permission(_ context.Context, request Permission) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.thread.assistant()
	s.thread.Messages[index].Parts = append(s.thread.Messages[index].Parts, Part{
		Kind:       PartPermission,
		Permission: &request,
	})
}

// ResolvePermission records the answer on the card in place: the part keeps its
// position and its content, so the browser only sees the decision change.
func (s *ThreadState) ResolvePermission(requestID, decision string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.thread.Messages {
		parts := s.thread.Messages[i].Parts
		for j := range parts {
			if parts[j].Kind == PartPermission && parts[j].Permission != nil &&
				parts[j].Permission.RequestID == requestID {
				parts[j].Permission.Decision = decision
				return true
			}
		}
	}
	return false
}

// apply folds one variant. Every variant has its own function, so the analysis's
// mapping table is readable one row at a time and a test can drive a single row
// without scripting a whole session.
func (s *ThreadState) apply(update acp.SessionUpdate) {
	switch {
	case update.UserMessageChunk != nil:
		s.applyUserChunk(update.UserMessageChunk)
	case update.AgentMessageChunk != nil:
		s.applyAgentChunk(update.AgentMessageChunk)
	case update.AgentThoughtChunk != nil:
		s.applyThoughtChunk(update.AgentThoughtChunk)
	case update.ToolCall != nil:
		s.applyToolCall(update.ToolCall)
	case update.ToolCallUpdate != nil:
		s.applyToolCallUpdate(update.ToolCallUpdate)
	case update.Plan != nil:
		s.applyPlan(update.Plan)
	case update.PlanUpdate != nil:
		s.applyPlanUpdate(update.PlanUpdate)
	case update.PlanRemoved != nil:
		s.applyPlanRemoved(update.PlanRemoved)
	case update.AvailableCommandsUpdate != nil:
		s.applyCommands(update.AvailableCommandsUpdate)
	case update.CurrentModeUpdate != nil:
		s.applyMode(update.CurrentModeUpdate)
	case update.ConfigOptionUpdate != nil:
		s.applyConfig(update.ConfigOptionUpdate)
	case update.SessionInfoUpdate != nil:
		s.applyInfo(update.SessionInfoUpdate)
	case update.UsageUpdate != nil:
		s.applyUsage(update.UsageUpdate)
	default:
		// A variant this build does not model, or one a future protocol adds:
		// dropped, never fatal. A chat that dies on an update it did not expect is
		// worse than one that ignores it.
	}
}

func (s *ThreadState) applyUserChunk(chunk *acp.SessionUpdateUserMessageChunk) {
	text, id := contentText(chunk.Content), messageID(chunk.MessageId)
	if index, ok := s.provisional(id); ok {
		s.thread.Messages[index].Parts = []Part{{Kind: PartText, Text: text}}
		s.thread.Messages[index].Provisional = false
		return
	}
	s.appendText(RoleUser, PartText, text, id)
}

// provisional returns the user message waiting for the agent's echo of it.
func (s *ThreadState) provisional(id string) (int, bool) {
	for i := range s.thread.Messages {
		message := s.thread.Messages[i]
		if message.Provisional && (id == "" || message.ID == id) {
			return i, true
		}
	}
	return 0, false
}

// openUserMessage writes the user's own text into the thread as the prompt goes out
// and returns the id it sent, so the agent's echo lands on the same message.
func (s *ThreadState) openUserMessage(text string) string {
	s.mu.Lock()
	id := fmt.Sprintf("user-%d", len(s.thread.Messages)+1)
	s.thread.Messages = append(s.thread.Messages, Message{
		ID:          id,
		Role:        RoleUser,
		Provisional: true,
		Parts:       []Part{{Kind: PartText, Text: text}},
	})
	s.mu.Unlock()
	s.notify()
	return id
}

// notify hands a snapshot to the listener, if one is registered.
func (s *ThreadState) notify() {
	s.mu.Lock()
	snapshot, listener := cloneThread(s.thread), s.listener
	s.mu.Unlock()
	if listener != nil {
		listener(snapshot)
	}
}

func (s *ThreadState) applyAgentChunk(chunk *acp.SessionUpdateAgentMessageChunk) {
	s.appendText(RoleAssistant, PartText, contentText(chunk.Content), messageID(chunk.MessageId))
}

func (s *ThreadState) applyThoughtChunk(chunk *acp.SessionUpdateAgentThoughtChunk) {
	s.appendText(RoleAssistant, PartThought, contentText(chunk.Content), messageID(chunk.MessageId))
}

func (s *ThreadState) applyToolCall(call *acp.SessionUpdateToolCall) {
	if part := s.findToolCall(string(call.ToolCallId)); part != nil {
		part.Title, part.Kind, part.Status = call.Title, string(call.Kind), string(call.Status)
		part.Input, part.Output = call.RawInput, call.RawOutput
		return
	}
	s.appendToolCall(&ToolCall{
		ID:     string(call.ToolCallId),
		Title:  call.Title,
		Kind:   string(call.Kind),
		Status: string(call.Status),
		Input:  call.RawInput,
		Output: call.RawOutput,
	})
}

func (s *ThreadState) applyToolCallUpdate(update *acp.SessionToolCallUpdate) {
	part := s.findToolCall(string(update.ToolCallId))
	if part == nil {
		// An update for a call this client never saw: the protocol makes the id the
		// identity, so the part is opened rather than the update dropped.
		s.appendToolCall(&ToolCall{ID: string(update.ToolCallId)})
		part = s.findToolCall(string(update.ToolCallId))
	}
	if part == nil {
		return
	}
	if update.Title != nil {
		part.Title = *update.Title
	}
	if update.Kind != nil {
		part.Kind = string(*update.Kind)
	}
	if update.Status != nil {
		part.Status = string(*update.Status)
	}
	if update.RawInput != nil {
		part.Input = update.RawInput
	}
	if update.RawOutput != nil {
		part.Output = update.RawOutput
	}
}

func (s *ThreadState) applyPlan(plan *acp.SessionUpdatePlan) {
	s.thread.Plan = &Plan{Entries: planEntries(plan.Entries)}
}

func (s *ThreadState) applyPlanUpdate(update *acp.SessionPlanUpdate) {
	raw, err := json.Marshal(update.Plan)
	if err != nil {
		return
	}
	plan := &Plan{Raw: raw}
	if items := update.Plan.Items; items != nil {
		plan.ID = string(items.Id)
		plan.Entries = planEntries(items.Entries)
		plan.Raw = nil
	}
	s.thread.Plan = plan
}

func (s *ThreadState) applyPlanRemoved(*acp.SessionUpdatePlanRemoved) {
	s.thread.Plan = nil
}

func (s *ThreadState) applyCommands(update *acp.SessionAvailableCommandsUpdate) {
	commands := make([]SlashCommand, 0, len(update.AvailableCommands))
	for _, command := range update.AvailableCommands {
		commands = append(commands, SlashCommand{Name: command.Name, Description: command.Description})
	}
	s.thread.Commands = commands
}

func (s *ThreadState) applyMode(update *acp.SessionCurrentModeUpdate) {
	s.thread.Mode = string(update.CurrentModeId)
}

func (s *ThreadState) applyConfig(update *acp.SessionConfigOptionUpdate) {
	config := make([]json.RawMessage, 0, len(update.ConfigOptions))
	for _, option := range update.ConfigOptions {
		// The option union is unstable and carries a select or a boolean; the
		// browser shows what the agent sent, so it is kept verbatim.
		if raw, err := json.Marshal(option); err == nil {
			config = append(config, raw)
		}
	}
	s.thread.Config = config
}

func (s *ThreadState) applyInfo(update *acp.SessionSessionInfoUpdate) {
	info := &SessionInfo{}
	if s.thread.Info != nil {
		*info = *s.thread.Info
	}
	if update.Title != nil {
		info.Title = *update.Title
	}
	if update.UpdatedAt != nil {
		info.UpdatedAt = *update.UpdatedAt
	}
	s.thread.Info = info
}

func (s *ThreadState) applyUsage(update *acp.SessionUsageUpdate) {
	usage := &Usage{Used: update.Used, Size: update.Size}
	if update.Cost != nil {
		usage.Amount, usage.Currency = update.Cost.Amount, update.Cost.Currency
	}
	s.thread.Usage = usage
}

// foldUsage records the usage a finished turn reported, which the protocol sends
// on the prompt response rather than as a session update.
func (s *ThreadState) foldUsage(usage *acp.Usage) {
	if usage == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.thread.Usage == nil {
		s.thread.Usage = &Usage{}
	}
	s.thread.Usage.Size = usage.TotalTokens
}

// appendText adds text to the message's last part when it has the same kind, and
// opens a new part otherwise. That one rule is what makes a chunk arriving after a
// tool_call_update appear as new text after the tool call rather than as more text
// before it.
func (s *ThreadState) appendText(role, kind, text, id string) {
	index := s.message(role, id)
	parts := s.thread.Messages[index].Parts
	if n := len(parts); n > 0 && parts[n-1].Kind == kind {
		parts[n-1].Text += text
		return
	}
	s.thread.Messages[index].Parts = append(parts, Part{Kind: kind, Text: text})
}

// message returns the index of the message to write into: the one the chunk names,
// else the last message of that role, else a new one.
func (s *ThreadState) message(role, id string) int {
	if id != "" {
		for i := range s.thread.Messages {
			if s.thread.Messages[i].ID == id {
				return i
			}
		}
	}
	if n := len(s.thread.Messages); n > 0 {
		last := s.thread.Messages[n-1]
		if last.Role == role && (id == "" || last.ID == "") {
			if id != "" {
				s.thread.Messages[n-1].ID = id
			}
			return n - 1
		}
	}
	s.thread.Messages = append(s.thread.Messages, Message{
		ID:   firstNonEmpty(id, fmt.Sprintf("msg-%d", len(s.thread.Messages)+1)),
		Role: role,
	})
	return len(s.thread.Messages) - 1
}

// assistant returns the index of the assistant message the agent is streaming
// into, opening one when the last message is not an assistant message. It is the
// message a permission card belongs to.
func (t *Thread) assistant() int {
	if n := len(t.Messages); n > 0 && t.Messages[n-1].Role == RoleAssistant {
		return n - 1
	}
	t.Messages = append(t.Messages, Message{
		ID:   fmt.Sprintf("msg-%d", len(t.Messages)+1),
		Role: RoleAssistant,
	})
	return len(t.Messages) - 1
}

// appendToolCall opens a tool-call part on the assistant turn.
func (s *ThreadState) appendToolCall(call *ToolCall) {
	index := s.thread.assistant()
	s.thread.Messages[index].Parts = append(s.thread.Messages[index].Parts, Part{
		Kind:     PartToolCall,
		ToolCall: call,
	})
}

// findToolCall returns the part of a tool call, wherever in the thread it is.
func (s *ThreadState) findToolCall(id string) *ToolCall {
	for i := range s.thread.Messages {
		parts := s.thread.Messages[i].Parts
		for j := range parts {
			if parts[j].ToolCall != nil && parts[j].ToolCall.ID == id {
				return parts[j].ToolCall
			}
		}
	}
	return nil
}

func contentText(content acp.ContentBlock) string {
	if content.Text == nil {
		return ""
	}
	return content.Text.Text
}

func messageID(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

func planEntries(entries []acp.PlanEntry) []PlanEntry {
	out := make([]PlanEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, PlanEntry{
			Content:  entry.Content,
			Priority: string(entry.Priority),
			Status:   string(entry.Status),
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// cloneThread copies a thread deeply enough that a listener may marshal it while
// the fold keeps mutating the original.
func cloneThread(thread Thread) Thread {
	clone := thread
	clone.Messages = make([]Message, len(thread.Messages))
	for i, message := range thread.Messages {
		clone.Messages[i] = message
		clone.Messages[i].Parts = make([]Part, len(message.Parts))
		for j, part := range message.Parts {
			clone.Messages[i].Parts[j] = part
			if part.ToolCall != nil {
				call := *part.ToolCall
				clone.Messages[i].Parts[j].ToolCall = &call
			}
			if part.Permission != nil {
				permission := *part.Permission
				clone.Messages[i].Parts[j].Permission = &permission
			}
		}
	}
	if thread.Plan != nil {
		plan := *thread.Plan
		plan.Entries = append([]PlanEntry(nil), thread.Plan.Entries...)
		plan.Raw = append(json.RawMessage(nil), thread.Plan.Raw...)
		clone.Plan = &plan
	}
	clone.Commands = append([]SlashCommand(nil), thread.Commands...)
	clone.Config = append([]json.RawMessage(nil), thread.Config...)
	if thread.Info != nil {
		info := *thread.Info
		clone.Info = &info
	}
	if thread.Usage != nil {
		usage := *thread.Usage
		clone.Usage = &usage
	}
	return clone
}
