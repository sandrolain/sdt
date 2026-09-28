package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBrokerPublish(t *testing.T) {
	b := newBroker()
	ch, unsubscribe := b.subscribe()
	defer unsubscribe()

	b.publish([]string{"context/a.md"})
	select {
	case data := <-ch:
		if !strings.Contains(string(data), "context/a.md") {
			t.Errorf("payload = %s", data)
		}
	case <-time.After(time.Second):
		t.Fatal("no event published")
	}
}

// safeRecorder wraps httptest.ResponseRecorder with a mutex so a test goroutine
// can inspect the body while the handler goroutine writes to it race-free.
type safeRecorder struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
}

func newSafeRecorder() *safeRecorder {
	return &safeRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *safeRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(b)
}

func (r *safeRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ResponseRecorder.Flush()
}

// body returns the response body under the lock.
func (r *safeRecorder) body() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Body.String()
}

func TestHandleEventsStreamsChanges(t *testing.T) {
	root := makeCorpus(t)
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := newSafeRecorder()

	done := make(chan struct{})
	go func() {
		s.handleEvents(rec, req)
		close(done)
	}()

	// give the handler a moment to subscribe, then publish
	deadline := time.After(time.Second)
	for len(rec.body()) == 0 {
		select {
		case <-deadline:
			t.Fatal("no SSE preamble")
		case <-time.After(2 * time.Millisecond):
		}
	}
	s.broker.publish([]string{"context/notes/x.md"})
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	body := rec.body()
	if !strings.Contains(body, "event: change") || !strings.Contains(body, "context/notes/x.md") {
		t.Errorf("stream body missing change event:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestWatchCorpusDebouncesAndFilters(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "context", "notes", "a.md"), "a")
	if err := os.MkdirAll(filepath.Join(root, "context", "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}

	events := make(chan []string, 4)
	w, err := watchCorpus(root, filepath.Join(root, "context"), 20*time.Millisecond, func(paths []string) {
		events <- paths
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	// excluded path: must not trigger
	mustWrite(t, filepath.Join(root, "context", "tmp", "noise.md"), "noise")
	// included path: must trigger
	mustWrite(t, filepath.Join(root, "context", "notes", "b.md"), "b")

	select {
	case paths := <-events:
		if len(paths) != 1 || paths[0] != "context/notes/b.md" {
			t.Errorf("paths = %v, want [context/notes/b.md]", paths)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no change event")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
