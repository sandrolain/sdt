package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// ErrOutsideRoot is the confinement's refusal: the path resolves outside the
// project root, so the bridge leaves it alone.
var ErrOutsideRoot = errors.New("acpbridge: the path resolves outside the project root")

// Sink is where the client publishes what the browser must see: the agent's
// session updates, a permission request waiting for the user, and a question the
// user has to answer. The thread fold implements it (phase 3); a bridge with no sink
// discards them, which is what a handshake-only test needs.
type Sink interface {
	// SessionUpdate receives one update from the agent, in arrival order.
	SessionUpdate(ctx context.Context, notification acp.SessionNotification) error
	// Permission announces a request the user has to answer. It is called once per
	// request, when that request becomes the current one; the answer comes back
	// through PermissionQueue.Answer.
	Permission(ctx context.Context, request Permission)
	// Elicitation announces a question the user has to answer. It is called once per
	// question, when it becomes current; the answer comes back through
	// ElicitationQueue.Answer.
	Elicitation(ctx context.Context, request Elicitation)
	// ElicitationComplete tells the thread that a URL question finished, which is the
	// protocol saying so rather than the reader closing the card.
	ElicitationComplete(ctx context.Context, elicitationID string)
}

// Permission is one permission request as the browser renders it: which tool, its
// title and its raw input, under the id the answer travels with.
type Permission struct {
	RequestID  string `json:"requestId"`
	SessionID  string `json:"sessionId"`
	ToolCallID string `json:"toolCallId"`
	ToolKind   string `json:"toolKind,omitempty"`
	Title      string `json:"title,omitempty"`
	Input      any    `json:"input,omitempty"`
	// Decision is the user's answer, recorded on the card in place when it arrives
	// (DecisionAllowed or DecisionDenied); empty while the request is waiting.
	Decision string `json:"decision,omitempty"`
}

// askWait is one request waiting for its answer. `dropped` is closed when the wait is
// cancelled rather than answered, so the goroutine blocked on it is released at once
// instead of waiting for a context it may not have.
type askWait[Req any, Ans any] struct {
	request Req
	answer  chan Ans
	dropped chan struct{}
}

// askQueue brokers the cards of one connection: permission requests and questions share
// it, because they have the same life — one current at a time, a second queued rather
// than dropped, an answer that promotes the next, and a session that ends releasing what
// it held. The two typed faces below keep their own vocabulary; only the mechanics live
// here, so the two cannot drift.
//
// The three accessors are how the queue names a request without an interface the request
// types would have to implement: the id the browser answers with, the session that owns
// it, and the request with its id assigned.
type askQueue[Req any, Ans any] struct {
	mu      sync.Mutex
	counter int
	prefix  string
	queue   []*askWait[Req, Ans]
	publish func(context.Context, Req)
	id      func(Req) string
	session func(Req) string
	tag     func(Req, string) Req
}

func newAskQueue[Req any, Ans any](
	prefix string,
	publish func(context.Context, Req),
	id func(Req) string,
	session func(Req) string,
	tag func(Req, string) Req,
) *askQueue[Req, Ans] {
	return &askQueue[Req, Ans]{prefix: prefix, publish: publish, id: id, session: session, tag: tag}
}

// ask makes the request current — or queues it behind one already waiting — and blocks
// until the user answers or ctx ends. The error is ctx's, or ErrCancelled when the wait
// was dropped.
func (q *askQueue[Req, Ans]) ask(ctx context.Context, request Req) (Ans, error) {
	q.mu.Lock()
	q.counter++
	request = q.tag(request, fmt.Sprintf("%s-%d", q.prefix, q.counter))
	wait := &askWait[Req, Ans]{
		request: request,
		answer:  make(chan Ans, 1),
		dropped: make(chan struct{}),
	}
	q.queue = append(q.queue, wait)
	current := len(q.queue) == 1
	q.mu.Unlock()

	if current && q.publish != nil {
		q.publish(ctx, request)
	}
	var zero Ans
	select {
	case answer := <-wait.answer:
		return answer, nil
	case <-wait.dropped:
		return zero, ErrCancelled
	case <-ctx.Done():
		q.forget(wait)
		return zero, ctx.Err()
	}
}

// answer resolves the request with this id and reports whether it was waiting. The
// request after it becomes current, so the user sees the queue one card at a time.
func (q *askQueue[Req, Ans]) answer(requestID string, answer Ans) bool {
	q.mu.Lock()
	index := -1
	for i, wait := range q.queue {
		if q.id(wait.request) == requestID {
			index = i
			break
		}
	}
	if index < 0 {
		q.mu.Unlock()
		return false
	}
	answered := q.queue[index]
	q.queue = append(q.queue[:index:index], q.queue[index+1:]...)
	var next *askWait[Req, Ans]
	if index == 0 && len(q.queue) > 0 {
		next = q.queue[0]
	}
	q.mu.Unlock()

	answered.answer <- answer
	if next != nil && q.publish != nil {
		q.publish(context.Background(), next.request)
	}
	return true
}

// cancelSession drops every wait of one session and reports how many it dropped: the
// browser that would have answered them is gone, and a card nobody can answer must not
// leave the agent blocked. The agent receives the protocol's own cancellation, because
// the request's context is the one that ended.
func (q *askQueue[Req, Ans]) cancelSession(sessionID string) int {
	q.mu.Lock()
	kept := make([]*askWait[Req, Ans], 0, len(q.queue))
	var dropped []*askWait[Req, Ans]
	for _, wait := range q.queue {
		if q.session(wait.request) == sessionID {
			dropped = append(dropped, wait)
			continue
		}
		kept = append(kept, wait)
	}
	wasCurrent := len(kept) > 0 && len(q.queue) > 0 && q.queue[0] != kept[0]
	q.queue = kept
	var next *askWait[Req, Ans]
	if wasCurrent && len(kept) > 0 {
		next = kept[0]
	}
	q.mu.Unlock()

	for _, wait := range dropped {
		close(wait.dropped)
	}
	if next != nil && q.publish != nil {
		q.publish(context.Background(), next.request)
	}
	return len(dropped)
}

// forget drops a wait whose context ended, and promotes the next one.
func (q *askQueue[Req, Ans]) forget(wait *askWait[Req, Ans]) {
	q.mu.Lock()
	index := -1
	for i, queued := range q.queue {
		if queued == wait {
			index = i
			break
		}
	}
	if index < 0 {
		q.mu.Unlock()
		return
	}
	q.queue = append(q.queue[:index:index], q.queue[index+1:]...)
	var next *askWait[Req, Ans]
	if index == 0 && len(q.queue) > 0 {
		next = q.queue[0]
	}
	q.mu.Unlock()
	if next != nil && q.publish != nil {
		q.publish(context.Background(), next.request)
	}
}

// PermissionQueue brokers the permission requests of one connection.
type PermissionQueue struct {
	q *askQueue[Permission, bool]
}

// NewPermissionQueue returns a queue that announces the current request through
// publish. A nil publish keeps the queue silent, which is what a test that only
// exercises the answers needs.
func NewPermissionQueue(publish func(context.Context, Permission)) *PermissionQueue {
	return &PermissionQueue{q: newAskQueue[Permission, bool]("perm", publish,
		func(request Permission) string { return request.RequestID },
		func(request Permission) string { return request.SessionID },
		func(request Permission, id string) Permission { request.RequestID = id; return request },
	)}
}

// Ask makes the request current — or queues it behind one already waiting — and
// blocks until the user answers or ctx ends. The error is ctx's.
func (q *PermissionQueue) Ask(ctx context.Context, request Permission) (bool, error) {
	return q.q.ask(ctx, request)
}

// Answer resolves the request with this id and reports whether it was waiting.
func (q *PermissionQueue) Answer(requestID string, allow bool) bool {
	return q.q.answer(requestID, allow)
}

// CancelSession drops every wait of one session and reports how many it dropped.
func (q *PermissionQueue) CancelSession(sessionID string) int {
	return q.q.cancelSession(sessionID)
}

// The two shapes a question can take: a form built from a JSON schema, or a URL the
// reader is sent to.
const (
	ElicitationForm = "form"
	ElicitationURL  = "url"
)

// The user's answer to a question, as the card records it.
const (
	DecisionAccepted  = "accepted"
	DecisionDeclined  = "declined"
	DecisionCancelled = "cancelled"
)

// The actions a browser sends back, which are the protocol's own words.
const (
	ActionAccept  = "accept"
	ActionDecline = "decline"
	ActionCancel  = "cancel"
)

// Elicitation is one question the agent asks, as the browser renders it. The schema is
// the requested schema verbatim — the protocol's shape, not a second model of it — so a
// property this build cannot render is still visible to the reader and to the panel.
type Elicitation struct {
	RequestID string `json:"requestId"`
	SessionID string `json:"sessionId"`
	// Mode is ElicitationForm or ElicitationURL.
	Mode    string `json:"mode"`
	Message string `json:"message"`
	// Schema is the form's requested schema (object with properties and required).
	Schema map[string]any `json:"schema,omitempty"`
	// URL is set for the url mode, with the id the protocol completes it by.
	URL           string `json:"url,omitempty"`
	ElicitationID string `json:"elicitationId,omitempty"`
	// Decision is the reader's answer, recorded on the card in place; empty while the
	// request is waiting, and DecisionCancelled when nobody could answer it.
	Decision string `json:"decision,omitempty"`
	// Content is the accepted form's values, if any.
	Content map[string]any `json:"content,omitempty"`
	// Completed is the protocol's completion of a URL question.
	Completed bool `json:"completed,omitempty"`
}

// ElicitationAnswer is what the browser sends back for one question.
type ElicitationAnswer struct {
	// Action is ActionAccept, ActionDecline or ActionCancel.
	Action  string
	Content map[string]any
}

// ElicitationQueue brokers the questions of one connection.
type ElicitationQueue struct {
	q *askQueue[Elicitation, ElicitationAnswer]
}

// NewElicitationQueue returns a queue that announces the current question through
// publish. A nil publish keeps it silent, which a test that only answers needs.
func NewElicitationQueue(publish func(context.Context, Elicitation)) *ElicitationQueue {
	return &ElicitationQueue{q: newAskQueue[Elicitation, ElicitationAnswer]("elicit", publish,
		func(request Elicitation) string { return request.RequestID },
		func(request Elicitation) string { return request.SessionID },
		func(request Elicitation, id string) Elicitation { request.RequestID = id; return request },
	)}
}

// Ask makes the question current — or queues it behind one already waiting — and blocks
// until the reader answers or ctx ends.
func (q *ElicitationQueue) Ask(ctx context.Context, request Elicitation) (ElicitationAnswer, error) {
	return q.q.ask(ctx, request)
}

// Answer resolves the question with this id and reports whether it was waiting.
func (q *ElicitationQueue) Answer(requestID string, answer ElicitationAnswer) bool {
	return q.q.answer(requestID, answer)
}

// CancelSession drops every question of one session and reports how many it dropped.
func (q *ElicitationQueue) CancelSession(sessionID string) int {
	return q.q.cancelSession(sessionID)
}

// client is the acp.Client implementation: the policy this wave decided on. No
// method is a pass-through — the file pair is confined to the root, a permission
// request suspends for the user, and the terminal family refuses.
type client struct {
	root  string
	sink  Sink
	perms *PermissionQueue
	// elicitations is the question queue, and currentSession names the conversation a
	// question belongs to: the protocol's request carries no session id, so the bridge
	// supplies the one it holds when the question arrives.
	elicitations   *ElicitationQueue
	currentSession func() string
}

var _ acp.Client = (*client)(nil)

// The question methods are not part of acp.Client — the SDK routes them only to a client
// that implements the unstable interface, and answers MethodNotFound otherwise — so the
// contract is pinned here: deleting or renaming a method becomes a build failure instead
// of an agent's question silently disappearing.
var _ interface {
	UnstableCreateElicitation(context.Context, acp.UnstableCreateElicitationRequest) (acp.UnstableCreateElicitationResponse, error)
	UnstableCompleteElicitation(context.Context, acp.UnstableCompleteElicitationNotification) error
} = (*client)(nil)

// resolveProjectPath is the single confinement point for the file callbacks. Given
// the project root and the path the agent asked for, it returns the absolute path
// to act on, or ErrOutsideRoot.
//
// Both the root and the request are resolved through their symlinks — the request
// through its deepest existing ancestor, because a *write* legitimately targets a
// file that does not exist yet. The caller acts on the path this returns and never
// on the raw request, so a path that cleans to something inside the root while the
// kernel would resolve it elsewhere cannot be reached: the checked path and the
// used path are the same string.
//
// The confinement is not atomic with the open that follows it, so a symlink
// swapped in that window would not be caught. That is accepted for this wave: the
// agent already has write access inside the root by policy, and the boundary here
// is the root, not a race with the agent.
func resolveProjectPath(root, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", fmt.Errorf("%w: the path is empty", ErrOutsideRoot)
	}
	// ACP hands the client absolute paths; a relative one would silently mean
	// something different here than it does to the agent, so it is refused rather
	// than guessed at.
	if !filepath.IsAbs(requested) {
		return "", fmt.Errorf("%w: %q is not absolute", ErrOutsideRoot, requested)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("%w: project root %q: %v", ErrOutsideRoot, root, err)
	}
	resolved, err := resolveDeepest(requested)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrOutsideRoot, requested, err)
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q resolves to %q, which is outside %q",
			ErrOutsideRoot, requested, resolved, resolvedRoot)
	}
	return resolved, nil
}

// resolveDeepest resolves the symlinks of the longest existing prefix of path and
// appends the names that do not exist yet, so a write to a new file under a
// symlinked directory is resolved the way the kernel would resolve it.
func resolveDeepest(path string) (string, error) {
	path = filepath.Clean(path)
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append([]string{filepath.Base(path)}, missing...)
		path = parent
	}
}

func (c *client) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	path, err := resolveProjectPath(c.root, params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	content, err := readWindow(path, params.Line, params.Limit)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("%w: reading %q: %v", ErrOutsideRoot, params.Path, err)
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

// readWindow reads the file, honouring the protocol's optional 1-based line window.
// A window that is asked for and answered with the whole file would be a silent
// wrong answer, so the window is applied here.
func readWindow(path string, line, limit *int) (string, error) {
	//nolint:gosec // path is resolveProjectPath's own output, checked against the root above
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if line == nil && limit == nil {
		return string(data), nil
	}
	lines := strings.Split(string(data), "\n")
	from := 0
	if line != nil && *line > 1 {
		from = *line - 1
	}
	if from > len(lines) {
		return "", nil
	}
	to := len(lines)
	if limit != nil && from+*limit < to {
		to = from + *limit
	}
	if to < from {
		to = from
	}
	return strings.Join(lines[from:to], "\n"), nil
}

func (c *client) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	path, err := resolveProjectPath(c.root, params.Path)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("%w: writing %q: %v", ErrOutsideRoot, params.Path, err)
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o600); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("%w: writing %q: %v", ErrOutsideRoot, params.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// RequestPermission suspends until the user answers: the request is published to
// the sink (one at a time) and the wait ends with the browser's decision, or with
// the protocol's own cancellation when the wait's context ends.
func (c *client) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	allow, err := c.perms.Ask(ctx, permissionFrom(params))
	if err != nil {
		// The connection died or the request was cancelled: the protocol has a word
		// for that, and the agent must not be left waiting for a card nobody will
		// see. The error is not propagated: a JSON-RPC error would tell the agent
		// less than the cancellation does.
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeCancelled(),
		}, nil
	}
	return acp.RequestPermissionResponse{Outcome: outcomeFor(params.Options, allow)}, nil
}

// outcomeFor maps the user's answer onto the vocabulary the *agent* chose. The
// bridge picks the allow or the reject option the agent named, and answers
// cancelled when it named none — it never invents an option id, because the agent
// is the one that can carry the decision out.
func outcomeFor(options []acp.PermissionOption, allow bool) acp.RequestPermissionOutcome {
	preferred := []acp.PermissionOptionKind{
		acp.PermissionOptionKindAllowOnce,
		acp.PermissionOptionKindAllowAlways,
	}
	if !allow {
		preferred = []acp.PermissionOptionKind{
			acp.PermissionOptionKindRejectOnce,
			acp.PermissionOptionKindRejectAlways,
		}
	}
	for _, kind := range preferred {
		for _, option := range options {
			if option.Kind == kind {
				return acp.NewRequestPermissionOutcomeSelected(option.OptionId)
			}
		}
	}
	return acp.NewRequestPermissionOutcomeCancelled()
}

func permissionFrom(params acp.RequestPermissionRequest) Permission {
	request := Permission{
		SessionID:  string(params.SessionId),
		ToolCallID: string(params.ToolCall.ToolCallId),
		Input:      params.ToolCall.RawInput,
	}
	if params.ToolCall.Kind != nil {
		request.ToolKind = string(*params.ToolCall.Kind)
	}
	if params.ToolCall.Title != nil {
		request.Title = *params.ToolCall.Title
	}
	return request
}

// UnstableCreateElicitation suspends until the reader answers the question the agent
// asks: the request is published as a card (one at a time, queued behind any other) and
// the wait ends with the reader's answer or with the protocol's own cancellation when
// the wait's context ends. The protocol's actions are mapped here, at the boundary; the
// card records the past tense.
func (c *client) UnstableCreateElicitation(ctx context.Context, params acp.UnstableCreateElicitationRequest) (acp.UnstableCreateElicitationResponse, error) {
	request := elicitationFrom(params)
	// The protocol's request is not tied to a session; the conversation the bridge holds
	// is the one the question belongs to, and naming it is what lets a conversation that
	// ends release its question.
	if c.currentSession != nil {
		request.SessionID = c.currentSession()
	}
	answer, err := c.elicitations.Ask(ctx, request)
	if err != nil {
		// The connection died or the request was cancelled: the agent must not be left
		// waiting for a card nobody will see, and a JSON-RPC error would tell it less than
		// the protocol's own cancellation.
		return acp.NewUnstableCreateElicitationResponseCancel(), nil
	}
	switch answer.Action {
	case ActionAccept:
		return acp.UnstableCreateElicitationResponse{
			Accept: &acp.UnstableCreateElicitationAccept{Action: ActionAccept, Content: answer.Content},
		}, nil
	case ActionDecline:
		return acp.NewUnstableCreateElicitationResponseDecline(), nil
	default:
		return acp.NewUnstableCreateElicitationResponseCancel(), nil
	}
}

// UnstableCompleteElicitation records that a URL question finished. It carries no
// session, so the sink marks the card the id names wherever it stands.
func (c *client) UnstableCompleteElicitation(ctx context.Context, notification acp.UnstableCompleteElicitationNotification) error {
	if c.sink == nil {
		return nil
	}
	c.sink.ElicitationComplete(ctx, string(notification.ElicitationId))
	return nil
}

// elicitationFrom converts the protocol's two request shapes into the card's one: the
// schema travels verbatim — its properties, the required names, its title and
// description — because a second model of it would lose whatever this build does not
// know how to render.
func elicitationFrom(params acp.UnstableCreateElicitationRequest) Elicitation {
	request := Elicitation{}
	switch {
	case params.Form != nil:
		request.Mode = ElicitationForm
		request.Message = params.Form.Message
		request.Schema = schemaMap(params.Form.RequestedSchema)
	case params.Url != nil:
		request.Mode = ElicitationURL
		request.Message = params.Url.Message
		request.URL = params.Url.Url
		request.ElicitationID = string(params.Url.ElicitationId)
	}
	return request
}

// schemaMap is the requested schema as the browser reads it: the protocol's own JSON,
// decoded into the generic map the wire carries.
func schemaMap(schema acp.UnstableElicitationSchema) map[string]any {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return decoded
}

func (c *client) SessionUpdate(ctx context.Context, notification acp.SessionNotification) error {
	if c.sink == nil {
		return nil
	}
	return c.sink.SessionUpdate(ctx, notification)
}

// The terminal family: refused for the whole wave, with no configuration path. A
// flag would make the reader's trust decision depend on a deployment detail.
func (c *client) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, ErrRefused
}

func (c *client) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, ErrRefused
}

func (c *client) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, ErrRefused
}

func (c *client) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, ErrRefused
}

func (c *client) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, ErrRefused
}
