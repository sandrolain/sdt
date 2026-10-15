package acpbridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
)

// recordingSink is the browser's stand-in: it records what the client publishes
// and can signal a test when a permission card appears, so a test waits for the
// card instead of sleeping.
type recordingSink struct {
	mu           sync.Mutex
	updates      []acp.SessionNotification
	permissions  []Permission
	onPermission func(Permission)
}

func (s *recordingSink) SessionUpdate(_ context.Context, notification acp.SessionNotification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, notification)
	return nil
}

func (s *recordingSink) Permission(_ context.Context, request Permission) {
	s.mu.Lock()
	s.permissions = append(s.permissions, request)
	notify := s.onPermission
	s.mu.Unlock()
	if notify != nil {
		notify(request)
	}
}

// The confinement matrix: what the policy refuses and what it allows, next to each
// other so the list cannot pass by refusing everything.
func TestResolveProjectPathConfinesToTheRoot(t *testing.T) {
	root := t.TempDir()
	// A second root under the same parent is genuinely outside the first, which a
	// sibling directory proves better than a hard-coded /tmp path.
	outside := t.TempDir()

	notes := filepath.Join(root, "context", "notes")
	if err := os.MkdirAll(notes, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	inside := filepath.Join(notes, "a.md")
	if err := os.WriteFile(inside, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	// A symlink inside the root that points outside it is the escape a naive
	// prefix check misses.
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// A symlink inside the root that points inside it must keep working.
	if err := os.Symlink(notes, filepath.Join(root, "notes-link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	allowed := []struct {
		name      string
		requested string
	}{
		{"a file inside the root", inside},
		{"a file that does not exist yet (a write)", filepath.Join(notes, "new.md")},
		{"through a symlink that stays inside", filepath.Join(root, "notes-link", "a.md")},
		{"the root itself", root},
	}
	for _, tc := range allowed {
		t.Run("allows "+tc.name, func(t *testing.T) {
			if _, err := resolveProjectPath(root, tc.requested); err != nil {
				t.Fatalf("resolveProjectPath(%q) = %v, want it allowed", tc.requested, err)
			}
		})
	}

	refused := []struct {
		name      string
		requested string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"relative", filepath.Join("context", "notes", "a.md")},
		{"a dot-dot escape", filepath.Join(root, "..", filepath.Base(outside), "secret.md")},
		{"an absolute path outside", secret},
		{"through a symlink out of the root", filepath.Join(root, "escape", "secret.md")},
		{"the escaping symlink itself", filepath.Join(root, "escape")},
		{"a parent of the root", filepath.Dir(root)},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			if _, err := resolveProjectPath(root, tc.requested); !errors.Is(err, ErrOutsideRoot) {
				t.Fatalf("resolveProjectPath(%q) = %v, want ErrOutsideRoot", tc.requested, err)
			}
		})
	}
}

func newTestClient(t *testing.T, root string, sink Sink) *client {
	t.Helper()
	if sink == nil {
		sink = &recordingSink{}
	}
	return &client{root: root, sink: sink, perms: NewPermissionQueue(sink.Permission)}
}

// The positive case and the refusals together: a read inside the root returns the
// content, a read outside it returns the typed refusal and touches nothing.
func TestReadTextFileIsConfinedToTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	inside := filepath.Join(root, "doc.md")
	if err := os.WriteFile(inside, []byte("line one\nline two\nline three\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("do not read me"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	ctx := context.Background()
	c := newTestClient(t, root, nil)

	got, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{Path: inside})
	if err != nil {
		t.Fatalf("ReadTextFile inside the root: %v", err)
	}
	if got.Content != "line one\nline two\nline three\n" {
		t.Fatalf("ReadTextFile returned %q", got.Content)
	}

	if _, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{Path: secret}); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("ReadTextFile outside the root = %v, want ErrOutsideRoot", err)
	}
}

// A window that is asked for and answered with the whole file would be a silent
// wrong answer, so the protocol's line window is honoured.
func TestReadTextFileHonoursTheLineWindow(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := newTestClient(t, root, nil)
	line, limit := 2, 2

	got, err := c.ReadTextFile(context.Background(), acp.ReadTextFileRequest{Path: path, Line: &line, Limit: &limit})
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got.Content != "two\nthree" {
		t.Fatalf("windowed read returned %q, want %q", got.Content, "two\nthree")
	}
}

// A write inside the root lands, and a write outside it is refused before
// anything is created: the policy's own positive case next to its refusal.
func TestWriteTextFileIsConfinedToTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	ctx := context.Background()
	c := newTestClient(t, root, nil)

	inside := filepath.Join(root, "context", "notes", "written.md")
	if _, err := c.WriteTextFile(ctx, acp.WriteTextFileRequest{Path: inside, Content: "the agent wrote this"}); err != nil {
		t.Fatalf("WriteTextFile inside the root: %v", err)
	}
	content, err := os.ReadFile(inside)
	if err != nil {
		t.Fatalf("the write did not land: %v", err)
	}
	if string(content) != "the agent wrote this" {
		t.Fatalf("the written file holds %q", content)
	}

	escape := filepath.Join(root, "..", filepath.Base(outside), "planted.md")
	if _, err := c.WriteTextFile(ctx, acp.WriteTextFileRequest{Path: escape, Content: "planted"}); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("WriteTextFile outside the root = %v, want ErrOutsideRoot", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "planted.md")); !os.IsNotExist(err) {
		t.Fatalf("a refused write created a file outside the root (stat err = %v)", err)
	}
}

// The card the user answers: what the agent asked is published, the wait ends with
// the answer, and the answer travels back as the option the agent offered.
func TestPermissionSuspendsUntilTheUserAnswers(t *testing.T) {
	kind := acp.ToolKind("edit")
	title := "Write context/notes/x.md"
	sink := &recordingSink{}
	cards := make(chan Permission, 4)
	sink.onPermission = func(request Permission) { cards <- request }
	c := newTestClient(t, t.TempDir(), sink)

	options := []acp.PermissionOption{
		{OptionId: "yes", Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow"},
		{OptionId: "no", Kind: acp.PermissionOptionKindRejectOnce, Name: "Reject"},
	}
	params := acp.RequestPermissionRequest{
		SessionId: "sess-1",
		ToolCall: acp.ToolCallUpdate{
			ToolCallId: "call-1",
			Kind:       &kind,
			Title:      &title,
			RawInput:   map[string]any{"path": "context/notes/x.md"},
		},
		Options: options,
	}

	type answer struct {
		response acp.RequestPermissionResponse
		err      error
	}
	done := make(chan answer, 1)
	go func() {
		response, err := c.RequestPermission(context.Background(), params)
		done <- answer{response, err}
	}()

	var card Permission
	select {
	case card = <-cards:
	case <-time.After(2 * time.Second):
		t.Fatal("the permission request was never published to the user")
	}
	if card.RequestID == "" {
		t.Fatal("the published request carries no id to answer")
	}
	if card.ToolKind != "edit" || card.Title != title {
		t.Fatalf("the card says %q/%q, want the tool's kind and title", card.ToolKind, card.Title)
	}
	if card.Input == nil {
		t.Fatal("the card carries no raw input for the user to judge")
	}

	// Nothing runs before the answer: the call is still suspended.
	select {
	case result := <-done:
		t.Fatalf("the agent was answered before the user was: %v", result.response)
	default:
	}

	if !c.perms.Answer(card.RequestID, true) {
		t.Fatal("answering the current request reported nothing waiting")
	}
	result := <-done
	if result.err != nil {
		t.Fatalf("RequestPermission: %v", result.err)
	}
	if result.response.Outcome.Selected == nil || result.response.Outcome.Selected.OptionId != "yes" {
		t.Fatalf("the approval did not select the agent's allow option: %+v", result.response.Outcome)
	}
}

func TestDeniedPermissionSelectsTheAgentsRejectOption(t *testing.T) {
	sink := &recordingSink{}
	cards := make(chan Permission, 4)
	sink.onPermission = func(request Permission) { cards <- request }
	c := newTestClient(t, t.TempDir(), sink)

	params := acp.RequestPermissionRequest{
		SessionId: "sess-1",
		ToolCall:  acp.ToolCallUpdate{ToolCallId: "call-1"},
		Options: []acp.PermissionOption{
			{OptionId: "yes", Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow"},
			{OptionId: "no", Kind: acp.PermissionOptionKindRejectOnce, Name: "Reject"},
		},
	}
	responses := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		response, _ := c.RequestPermission(context.Background(), params)
		responses <- response
	}()

	card := <-cards
	c.perms.Answer(card.RequestID, false)
	response := <-responses
	if response.Outcome.Selected == nil || response.Outcome.Selected.OptionId != "no" {
		t.Fatalf("the denial did not select the agent's reject option: %+v", response.Outcome)
	}
}

// Two requests: the thread shows one card at a time, and the second waits its turn
// rather than being dropped.
func TestASecondPermissionWaitsItsTurn(t *testing.T) {
	sink := &recordingSink{}
	cards := make(chan Permission, 4)
	sink.onPermission = func(request Permission) { cards <- request }
	c := newTestClient(t, t.TempDir(), sink)

	ask := func(title string) chan bool {
		done := make(chan bool, 1)
		go func() {
			allow, _ := c.perms.Ask(context.Background(), Permission{Title: title})
			done <- allow
		}()
		return done
	}
	first := ask("first")
	firstCard := <-cards
	second := ask("second")

	select {
	case extra := <-cards:
		t.Fatalf("the second request was published while the first was unanswered: %q", extra.Title)
	case <-time.After(200 * time.Millisecond):
	}

	if !c.perms.Answer(firstCard.RequestID, true) {
		t.Fatal("the first request was not waiting")
	}
	if !<-first {
		t.Fatal("the first request was denied")
	}
	select {
	case secondCard := <-cards:
		if secondCard.Title != "second" {
			t.Fatalf("the promoted card is %q, want the second request", secondCard.Title)
		}
		if !c.perms.Answer(secondCard.RequestID, false) {
			t.Fatal("the second request was not waiting")
		}
		if <-second {
			t.Fatal("the second request was allowed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the second request never became current after the first was answered")
	}
}

// A wait whose context ends does not leave the agent hanging and does not block
// the queue: the request is cancelled and the next one becomes current.
func TestAPermissionWithoutAnAnswerIsCancelled(t *testing.T) {
	sink := &recordingSink{}
	cards := make(chan Permission, 4)
	sink.onPermission = func(request Permission) { cards <- request }
	c := newTestClient(t, t.TempDir(), sink)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	response, err := c.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: "sess-1",
		ToolCall:  acp.ToolCallUpdate{ToolCallId: "call-1"},
		Options: []acp.PermissionOption{
			{OptionId: "yes", Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow"},
		},
	})
	if err != nil {
		t.Fatalf("a cancelled wait must still answer the agent: %v", err)
	}
	if response.Outcome.Cancelled == nil {
		t.Fatalf("a cancelled wait returned %+v, want the protocol's cancellation", response.Outcome)
	}
	if got := len(c.perms.queue); got != 0 {
		t.Fatalf("%d requests were left in the queue after the wait ended", got)
	}
}

// An agent that offers no option for the user's answer gets the protocol's
// cancellation: the bridge never invents an option id.
func TestPermissionWithoutAMatchingOptionIsCancelled(t *testing.T) {
	sink := &recordingSink{}
	cards := make(chan Permission, 4)
	sink.onPermission = func(request Permission) { cards <- request }
	c := newTestClient(t, t.TempDir(), sink)

	responses := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		response, _ := c.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess-1",
			ToolCall:  acp.ToolCallUpdate{ToolCallId: "call-1"},
			Options: []acp.PermissionOption{
				{OptionId: "later", Kind: "something_else", Name: "Later"},
			},
		})
		responses <- response
	}()
	card := <-cards
	c.perms.Answer(card.RequestID, true)
	response := <-responses
	if response.Outcome.Cancelled == nil {
		t.Fatalf("an unmatched answer returned %+v, want cancellation", response.Outcome)
	}
}

// The terminal family refuses, one assertion per method, and no configuration path
// turns one on.
func TestTerminalMethodsRefuse(t *testing.T) {
	c := newTestClient(t, t.TempDir(), nil)
	ctx := context.Background()
	cases := []struct {
		name string
		err  error
	}{
		{"create", errOf(func() error { _, err := c.CreateTerminal(ctx, acp.CreateTerminalRequest{}); return err })},
		{"kill", errOf(func() error { _, err := c.KillTerminal(ctx, acp.KillTerminalRequest{}); return err })},
		{"output", errOf(func() error { _, err := c.TerminalOutput(ctx, acp.TerminalOutputRequest{}); return err })},
		{"release", errOf(func() error { _, err := c.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{}); return err })},
		{"wait", errOf(func() error { _, err := c.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{}); return err })},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, ErrRefused) {
			t.Fatalf("the %s terminal method = %v, want ErrRefused", tc.name, tc.err)
		}
	}
}

// A session update reaches the sink, and a bridge with no sink is not a crash.
func TestSessionUpdateReachesTheSink(t *testing.T) {
	sink := &recordingSink{}
	c := newTestClient(t, t.TempDir(), sink)
	notification := acp.SessionNotification{SessionId: "sess-1"}
	if err := c.SessionUpdate(context.Background(), notification); err != nil {
		t.Fatalf("SessionUpdate: %v", err)
	}
	if got := len(sink.updates); got != 1 {
		t.Fatalf("the sink received %d updates, want 1", got)
	}

	silent := &client{root: t.TempDir(), perms: NewPermissionQueue(nil)}
	if err := silent.SessionUpdate(context.Background(), notification); err != nil {
		t.Fatalf("SessionUpdate with no sink: %v", err)
	}
}

func errOf(call func() error) error { return call() }
