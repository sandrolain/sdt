package main

import (
	"net/http"

	"github.com/sandrolain/sdt/internal/semantic"
)

// semanticStatus is the payload of /api/semantic/status: the two independent
// capabilities the web client probes before offering a semantic affordance.
type semanticStatus struct {
	// Enabled reports that the server semantic search branch is switched on and
	// operational (the search index exists and the model loaded); the palette's
	// Semantic toggle is hidden when it is false.
	Enabled bool `json:"enabled"`
	// Snapshot reports that a vector snapshot exists on disk, which is what the
	// docs semantic map needs. It is independent of Enabled: the map builds and
	// reads its own snapshot even when the search branch is off.
	Snapshot bool `json:"snapshot"`
}

// handleSemanticStatus serves /api/semantic/status, a read-only probe the web
// client uses to decide whether to offer semantic affordances. A corpus-less or
// model-less root reports Enabled false rather than advertising a toggle that
// could only degrade to lexical; the endpoint never errors.
func (s *server) handleSemanticStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, semanticStatus{
		Enabled:  s.semOpts.Enabled && s.semanticIndex() != nil,
		Snapshot: !semantic.LoadSnapshot(s.root).Empty(),
	})
}
