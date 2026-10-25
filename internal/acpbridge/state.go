package acpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Part kinds. The first four name the session/update variant that produced them, so the
// browser's renderer and the protocol stay checkable against each other; the last two come
// from the prompt response instead, which is where a turn's outcome arrives.
const (
	PartText       = "text"       // user_message_chunk, agent_message_chunk
	PartThought    = "thought"    // agent_thought_chunk
	PartToolCall   = "tool_call"  // tool_call, tool_call_update
	PartPermission = "permission" // a RequestPermission waiting for the user
	PartElicitation = "elicitation" // an elicitation/create waiting for the user
	PartError      = "error"      // the turn failed, in the agent's own words
	PartStop       = "stop"       // the turn stopped, for a reason worth telling
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
	SessionID string `json:"sessionId"`
	// Running marks a turn in flight: the user has sent a prompt and the agent has not
	// answered it yet. It is the thread's own fact rather than a part's, because the
	// question it answers — "is a turn happening now?" — is asked across messages, and a
	// snapshot that had to infer it from parts would guess at both ends of the turn.
	Running  bool              `json:"running,omitempty"`
	Messages []Message         `json:"messages"`
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
	//
	// The browser reads the flag as "this message has no answer yet" and shows the turn
	// as waiting; every outcome — the echo, a failure, a stop reason — clears it, and
	// that is what takes the wait away.
	Provisional bool   `json:"provisional,omitempty"`
	Parts       []Part `json:"parts"`
}

// Part is one piece of a message.
type Part struct {
	Kind       string      `json:"kind"`
	Text       string      `json:"text,omitempty"`
	ToolCall   *ToolCall   `json:"toolCall,omitempty"`
	Permission *Permission `json:"permission,omitempty"`
	// Elicitation is a question the agent asks, waiting for the reader's answer.
	Elicitation *Elicitation `json:"elicitation,omitempty"`
}

// ToolCall is one tool the agent ran, as the protocol reports it.
type ToolCall struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status,omitempty"`
	Input  any    `json:"input,omitempty"`
	Output any    `json:"output,omitempty"`
	// Content is what the tool itself produced, as text, when the protocol sends it as
	// content blocks rather than in the raw payload: the command's own output, which a
	// reader wants beside the status instead of only inside the record the panel keeps
	// behind a disclosure. A diff or a terminal reference is structure, not prose, and
	// stays in the raw record.
	Content string `json:"content,omitempty"`
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
	// loading is true while this thread is being loaded from the agent: a load replays
	// the conversation, and a replay folded onto a conversation we already hold would
	// show every message twice.
	loading bool
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
	if s.loading {
		// A load's replay, and this thread already holds the conversation: it is the
		// display source and is at least as complete as the replay, so the update is
		// dropped rather than folded on top of it.
		s.mu.Unlock()
		return nil
	}
	s.apply(notification.Update)
	s.mu.Unlock()
	s.notify()
	return nil
}

// WhileLoading runs fn with this thread's incoming updates dropped, unless the thread
// is still empty. That is the whole rule of a reattach: our thread is the display
// source, so a replay must not be added to it — and a thread that has nothing in it is
// the one case where the replay is the only account of the conversation, which is what
// a viewer that restarted finds.
func (s *ThreadState) WhileLoading(fn func() error) error {
	s.mu.Lock()
	// Dropped when we already hold the conversation, kept when we hold nothing: the
	// replay is a re-attachment of a history we already have, or the only account of
	// one we do not.
	s.loading = len(s.thread.Messages) > 0
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.loading = false
		s.mu.Unlock()
	}()
	return fn()
}

// Permission adds the card to the turn the agent is streaming, so the user answers
// it where the tool call appears.
func (s *ThreadState) Permission(_ context.Context, request Permission) {
	s.mu.Lock()
	index := s.thread.assistant()
	s.thread.Messages[index].Parts = append(s.thread.Messages[index].Parts, Part{
		Kind:       PartPermission,
		Permission: &request,
	})
	s.mu.Unlock()
	// A card the browser never hears about is a card nobody answers, so adding one
	// is a state change like any other.
	s.notify()
}

// Elicitation adds the question to the turn the agent is streaming, so the reader
// answers it where the agent asked.
func (s *ThreadState) Elicitation(_ context.Context, request Elicitation) {
	s.mu.Lock()
	index := s.thread.assistant()
	s.thread.Messages[index].Parts = append(s.thread.Messages[index].Parts, Part{
		Kind:        PartElicitation,
		Elicitation: &request,
	})
	s.mu.Unlock()
	s.notify()
}

// ResolveElicitation records the reader's answer on the question in place: the part keeps
// its position and its content, so the browser only sees the decision and the values
// change.
func (s *ThreadState) ResolveElicitation(requestID, decision string, content map[string]any) bool {
	s.mu.Lock()
	resolved := false
	for i := range s.thread.Messages {
		parts := s.thread.Messages[i].Parts
		for j := range parts {
			if parts[j].Kind == PartElicitation && parts[j].Elicitation != nil &&
				parts[j].Elicitation.RequestID == requestID {
				parts[j].Elicitation.Decision = decision
				parts[j].Elicitation.Content = content
				resolved = true
			}
		}
	}
	s.mu.Unlock()
	if resolved {
		s.notify()
	}
	return resolved
}

// CompleteElicitation marks a URL question finished, which is the protocol saying so.
func (s *ThreadState) CompleteElicitation(elicitationID string) bool {
	s.mu.Lock()
	completed := false
	for i := range s.thread.Messages {
		parts := s.thread.Messages[i].Parts
		for j := range parts {
			if parts[j].Kind == PartElicitation && parts[j].Elicitation != nil &&
				parts[j].Elicitation.ElicitationID == elicitationID {
				parts[j].Elicitation.Completed = true
				completed = true
			}
		}
	}
	s.mu.Unlock()
	if completed {
		s.notify()
	}
	return completed
}

// ResolvePermission records the answer on the card in place: the part keeps its
// position and its content, so the browser only sees the decision change.
func (s *ThreadState) ResolvePermission(requestID, decision string) bool {
	s.mu.Lock()
	resolved := false
	for i := range s.thread.Messages {
		parts := s.thread.Messages[i].Parts
		for j := range parts {
			if parts[j].Kind == PartPermission && parts[j].Permission != nil &&
				parts[j].Permission.RequestID == requestID {
				parts[j].Permission.Decision = decision
				resolved = true
			}
		}
	}
	s.mu.Unlock()
	if resolved {
		s.notify()
	}
	return resolved
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

// failTurn records that a turn produced no answer, as a part of the message that was sent:
// the reader's own message is where they look for the answer that did not come. The text is
// the agent's own sentence, never reworded — a refusal is the agent's to explain.
func (s *ThreadState) failTurn(id, text string) {
	s.record(id, PartError, text)
}

// endTurn records the reason a turn stopped, for the endings the reader would otherwise have
// to guess at: a turn that ran out of room is not the same as one still working.
func (s *ThreadState) endTurn(id, reason string) {
	s.record(id, PartStop, reason)
}

// settle takes a message out of the waiting state without recording anything on it: the turn
// is over, and a message whose turn ended is not still waiting even when the ending explains
// nothing. Most agents echo the user's own message at the start of a turn, which clears the
// flag by itself — but the protocol does not require that, so a turn that simply finished must
// not leave the reader watching a wait that nothing will ever clear.
func (s *ThreadState) settle(id string) {
	s.mu.Lock()
	if index, ok := s.messageIndex(id); ok {
		s.thread.Messages[index].Provisional = false
	}
	s.mu.Unlock()
	s.notify()
}

// record appends one outcome to the message a turn opened and takes that message out of the
// waiting state, so a message is never both waiting and answered.
func (s *ThreadState) record(id, kind, text string) {
	s.mu.Lock()
	if index, ok := s.messageIndex(id); ok {
		message := &s.thread.Messages[index]
		message.Parts = append(message.Parts, Part{Kind: kind, Text: text})
		message.Provisional = false
	}
	s.mu.Unlock()
	s.notify()
}

// messageIndex returns the position of the message with this id, and whether the thread holds
// one. The caller holds the lock.
func (s *ThreadState) messageIndex(id string) (int, bool) {
	for i := range s.thread.Messages {
		if s.thread.Messages[i].ID == id {
			return i, true
		}
	}
	return 0, false
}

// SetRunning records whether a turn is in flight and announces the change: the snapshot a
// browser reads is a whole thread, so a turn's span is a state change like any other and
// has to notify. A no-op when nothing changed keeps a repeated report from waking every
// listener for nothing.
func (s *ThreadState) SetRunning(running bool) {
	s.mu.Lock()
	if s.thread.Running == running {
		s.mu.Unlock()
		return
	}
	s.thread.Running = running
	s.mu.Unlock()
	s.notify()
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
		if content := toolContentText(call.Content); content != "" {
			part.Content = content
		}
		return
	}
	s.appendToolCall(&ToolCall{
		ID:      string(call.ToolCallId),
		Title:   call.Title,
		Kind:    string(call.Kind),
		Status:  string(call.Status),
		Input:   call.RawInput,
		Output:  call.RawOutput,
		Content: toolContentText(call.Content),
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
	if update.Content != nil {
		// The protocol replaces the collection rather than appending to it.
		part.Content = toolContentText(update.Content)
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
	s.applyConfigOptions(update.ConfigOptions)
}

// ConfigOptions folds the config options an agent answered a session open with. It is
// the open's own answer and not an update — the one place a model list is announced —
// so it takes the lock and announces the result the way an update does. An empty answer
// leaves the thread as it is rather than clearing what it already holds.
func (s *ThreadState) ConfigOptions(options []acp.SessionConfigOption) {
	if len(options) == 0 {
		return
	}
	s.mu.Lock()
	s.applyConfigOptions(options)
	s.mu.Unlock()
	s.notify()
}

// applyConfigOptions replaces the thread's configuration with what the agent sent. It is
// shared by the open's answer and the update variant so the two cannot disagree about the
// shape the browser renders.
func (s *ThreadState) applyConfigOptions(options []acp.SessionConfigOption) {
	config := make([]json.RawMessage, 0, len(options))
	for _, option := range options {
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

// toolContentText is the tool's own output, as the text blocks the protocol sent with it. A
// block that is a diff or a terminal reference carries structure rather than a sentence, and
// is left to the raw payload the browser keeps.
func toolContentText(blocks []acp.ToolCallContent) string {
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Content == nil {
			continue
		}
		if text := contentText(block.Content.Content); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
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
