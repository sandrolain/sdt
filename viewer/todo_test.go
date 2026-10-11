package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/todo"
)

// todoRoot builds a project root with context/ present and, when content is
// non-empty, a context/todo.yaml carrying it.
func todoRoot(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(todo.Path(root), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func getTodo(t *testing.T, root string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/todo", nil))
	return rec
}

func TestTodoRouteReturnsTheRegister(t *testing.T) {
	root := todoRoot(t, `items:
  - id: b-second
    text: "second"
    created: "2026-10-02"
  - id: a-first
    text: "first"
    created: "2026-10-01"
    done: true
    tags: [viewer]
`)
	rec := getTodo(t, root)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var got todoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(got.Items))
	}
	// the register order is preserved; the client orders, the route does not
	if got.Items[0].ID != "b-second" || got.Items[1].ID != "a-first" {
		t.Errorf("unexpected order: %+v", got.Items)
	}
	if !got.Items[1].Done || len(got.Items[1].Tags) != 1 || got.Items[1].Tags[0] != "viewer" {
		t.Errorf("second item lost its done/tags: %+v", got.Items[1])
	}
	// the wire keys are lowercase for the SPA (not the Go field names)
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw.Items[0]["id"]; !ok {
		t.Errorf("want a lowercase id key, got %v", raw.Items[0])
	}
}

func TestTodoRouteAbsentRegisterIsAnEmptyList(t *testing.T) {
	rec := getTodo(t, todoRoot(t, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
		t.Errorf("body = %s, want {\"items\":[]}", body)
	}
}

func TestTodoRouteMalformedRegisterIs500(t *testing.T) {
	rec := getTodo(t, todoRoot(t, "items: [oops\n"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), errTodoUnreadable) {
		t.Errorf("body = %s, want the opaque %q", rec.Body.String(), errTodoUnreadable)
	}
}
