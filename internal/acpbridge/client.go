package acpbridge

import (
	"context"
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
// session updates, and a permission request waiting for the user. The thread fold
// implements it (phase 3); a bridge with no sink discards both, which is what a
// handshake-only test needs.
type Sink interface {
	// SessionUpdate receives one update from the agent, in arrival order.
	SessionUpdate(ctx context.Context, notification acp.SessionNotification) error
	// Permission announces a request the user has to answer. It is called once per
	// request, when that request becomes the current one; the answer comes back
	// through PermissionQueue.Answer.
	Permission(ctx context.Context, request Permission)
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

// permissionWait is one request waiting for its answer. `dropped` is closed when
// the wait is cancelled rather than answered, so the goroutine blocked on it is
// released at once instead of waiting for a context it may not have.
type permissionWait struct {
	request Permission
	answer  chan bool
	dropped chan struct{}
}

// PermissionQueue brokers the permission requests of one connection. One request
// is current at a time — the thread shows one card — and a second waits its turn
// rather than being dropped, so an agent that asks twice gets two answers.
type PermissionQueue struct {
	mu      sync.Mutex
	counter int
	queue   []*permissionWait
	publish func(context.Context, Permission)
}

// NewPermissionQueue returns a queue that announces the current request through
// publish. A nil publish keeps the queue silent, which is what a test that only
// exercises the answers needs.
func NewPermissionQueue(publish func(context.Context, Permission)) *PermissionQueue {
	return &PermissionQueue{publish: publish}
}

// Ask makes the request current — or queues it behind one already waiting — and
// blocks until the user answers or ctx ends. The error is ctx's.
func (q *PermissionQueue) Ask(ctx context.Context, request Permission) (bool, error) {
	q.mu.Lock()
	q.counter++
	request.RequestID = fmt.Sprintf("perm-%d", q.counter)
	wait := &permissionWait{
		request: request,
		answer:  make(chan bool, 1),
		dropped: make(chan struct{}),
	}
	q.queue = append(q.queue, wait)
	current := len(q.queue) == 1
	q.mu.Unlock()

	if current && q.publish != nil {
		q.publish(ctx, request)
	}
	select {
	case allow := <-wait.answer:
		return allow, nil
	case <-wait.dropped:
		return false, ErrCancelled
	case <-ctx.Done():
		q.forget(wait)
		return false, ctx.Err()
	}
}

// Answer resolves the request with this id and reports whether it was waiting. The
// request after it becomes current, so the user sees the queue one card at a time.
func (q *PermissionQueue) Answer(requestID string, allow bool) bool {
	q.mu.Lock()
	index := -1
	for i, wait := range q.queue {
		if wait.request.RequestID == requestID {
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
	var next *permissionWait
	if index == 0 && len(q.queue) > 0 {
		next = q.queue[0]
	}
	q.mu.Unlock()

	answered.answer <- allow
	if next != nil && q.publish != nil {
		q.publish(context.Background(), next.request)
	}
	return true
}

// CancelSession drops every wait of one session and reports how many it dropped:
// the browser that would have answered them is gone, and a card nobody can answer
// must not leave the agent blocked. The agent receives the protocol's own
// cancellation, because the request's context is the one that ended.
func (q *PermissionQueue) CancelSession(sessionID string) int {
	q.mu.Lock()
	kept := make([]*permissionWait, 0, len(q.queue))
	var dropped []*permissionWait
	for _, wait := range q.queue {
		if wait.request.SessionID == sessionID {
			dropped = append(dropped, wait)
			continue
		}
		kept = append(kept, wait)
	}
	wasCurrent := len(kept) > 0 && len(q.queue) > 0 && q.queue[0] != kept[0]
	q.queue = kept
	var next *permissionWait
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
func (q *PermissionQueue) forget(wait *permissionWait) {
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
	var next *permissionWait
	if index == 0 && len(q.queue) > 0 {
		next = q.queue[0]
	}
	q.mu.Unlock()
	if next != nil && q.publish != nil {
		q.publish(context.Background(), next.request)
	}
}

// client is the acp.Client implementation: the policy this wave decided on. No
// method is a pass-through — the file pair is confined to the root, a permission
// request suspends for the user, and the terminal family refuses.
type client struct {
	root  string
	sink  Sink
	perms *PermissionQueue
}

var _ acp.Client = (*client)(nil)

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
