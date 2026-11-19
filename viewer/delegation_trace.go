package main

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/delegation"
)

const maxViewerDelegationTrace = 20 << 20

func (s *server) handleDelegationTrace(w http.ResponseWriter, r *http.Request) {
	if !s.delegationReadOnly {
		writeErr(w, http.StatusForbidden, "delegated task traces are disabled on a non-loopback bind host")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeErr(w, http.StatusMethodNotAllowed, "this endpoint takes a GET")
		return
	}
	if !agentOrigin(r) {
		writeErr(w, http.StatusForbidden, errAgentOrigin)
		return
	}

	taskID := r.URL.Query().Get("task")
	name := r.URL.Query().Get("artifact")
	if taskID == "" || !validTraceArtifactName(name) {
		http.NotFound(w, r)
		return
	}
	store := delegation.NewStore(s.root)
	task, err := store.Read(taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !taskHasTrace(task, name) {
		http.NotFound(w, r)
		return
	}
	data, err := store.ReadArtifact(taskID, name+".jsonl")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(data) > maxViewerDelegationTrace {
		writeErr(w, http.StatusRequestEntityTooLarge, "delegated task trace exceeds the viewer size limit")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`.jsonl"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	// #nosec G705 -- trace content is forced to nosniff text/plain and loaded in a sandboxed iframe.
	if _, err := w.Write(data); err != nil {
		return
	}
}

func validTraceArtifactName(name string) bool {
	if !strings.HasPrefix(name, "trace-") || filepath.Base(name) != name || !filepath.IsLocal(name) {
		return false
	}
	for _, r := range strings.TrimPrefix(name, "trace-") {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return len(name) > len("trace-")
}

func taskHasTrace(task delegation.Task, name string) bool {
	for _, artifact := range task.Artifacts {
		if artifact.Name == name {
			return true
		}
	}
	return false
}
