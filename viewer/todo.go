package main

// The TODO inbox is a CLI-owned register (context/todo.yaml). Decision 0027
// gives the viewer no write surface, so this route is read-only; internal/todo
// stays the single parser and mutator, so the viewer and the CLI cannot
// disagree about the register's shape.
//
// The wire shape is a viewer-local DTO with lowercase keys rather than
// internal/todo.Item directly: that struct carries YAML tags only, so
// marshalling it would emit capitalized Go field names — and changing those
// would also change `sdt context todo list --format json`, which marshals the
// same struct.

import (
	"log/slog"
	"net/http"

	"github.com/sandrolain/sdt/internal/todo"
)

// errTodoUnreadable is the opaque body for a malformed register; the real error
// is logged, never returned to the client (writeErr's contract).
const errTodoUnreadable = "todo register unreadable"

// todoItem is one inbox item on the wire. The register is returned whole: the
// client orders and filters, the route never does.
type todoItem struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Created string   `json:"created"`
	Source  string   `json:"source,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Done    bool     `json:"done"`
}

// todoResponse is the /api/todo payload.
type todoResponse struct {
	Items []todoItem `json:"items"`
}

// handleTodo serves the project's TODO register, read-only. A missing register
// is an empty inbox (200); a malformed one is a 500 with a stable message.
func (s *server) handleTodo(w http.ResponseWriter, _ *http.Request) {
	reg, err := todo.Load(s.root)
	if err != nil {
		slog.Error("sdtviewer: todo register", "err", err)
		writeErr(w, http.StatusInternalServerError, errTodoUnreadable)
		return
	}
	items := make([]todoItem, 0, len(reg.Items))
	for _, it := range reg.Items {
		items = append(items, todoItem{
			ID:      it.ID,
			Text:    it.Text,
			Created: it.Created,
			Source:  it.Source,
			Tags:    it.Tags,
			Done:    it.Done,
		})
	}
	writeJSON(w, http.StatusOK, todoResponse{Items: items})
}
