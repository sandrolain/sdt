package main

import (
	"log/slog"
	"net/http"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

// The corpus controlled vocabularies, served to the viewer so its search
// filters offer the values the corpus actually accepts instead of free text.
// The registers are parsed by internal/ctxvocab — the same code the CLI lint
// and `context new` use — so the palette cannot offer a value lint rejects.

// vocabResponse is the /api/vocab payload.
type vocabResponse struct {
	Objectives []string `json:"objectives"`
	Categories []string `json:"categories"`
	Topics     []string `json:"topics"`
}

// handleVocab serves GET /api/vocab: the objective groups and the category and
// topic registers. A missing register yields an empty list, never an error —
// the vocabulary is advisory in the corpus too.
func (s *server) handleVocab(w http.ResponseWriter, _ *http.Request) {
	objectives, err := ctxvocab.Objectives(s.root)
	if err != nil {
		slog.Warn("sdtviewer: objectives unavailable", "err", err)
	}
	categories, err := ctxvocab.LoadCategories(s.root)
	if err != nil {
		slog.Warn("sdtviewer: category register unavailable", "err", err)
	}
	topics, err := ctxvocab.LoadTopics(s.root)
	if err != nil {
		slog.Warn("sdtviewer: topic register unavailable", "err", err)
	}
	writeJSON(w, http.StatusOK, vocabResponse{
		Objectives: objectives,
		Categories: categories.Slugs(),
		Topics:     topics.Slugs(),
	})
}
