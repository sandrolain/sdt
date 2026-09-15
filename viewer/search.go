package main

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sandrolain/sdt/internal/search"
)

// loadSearch builds the in-memory bleve index over the markdown corpus at
// startup. A missing index (e.g. corpus-less root) is non-fatal: /api/search
// then degrades to an empty result.
func (s *server) loadSearch() error {
	ix, err := search.New(s.root)
	if err != nil {
		return err
	}
	s.srchMu.Lock()
	s.srch = ix
	s.srchMu.Unlock()
	return nil
}

// rebuildSearch rebuilds the index after a corpus change and publishes the
// changed paths to SSE subscribers. The old index is released.
func (s *server) rebuildSearch(paths []string) {
	ix, err := search.New(s.root)
	if err != nil {
		slog.Warn("sdtviewer: search rebuild failed", "err", err)
		return
	}
	s.srchMu.Lock()
	old := s.srch
	s.srch = ix
	s.srchMu.Unlock()
	if old != nil {
		if err := old.Close(); err != nil {
			slog.Warn("sdtviewer: search close failed", "err", err)
		}
	}
	slog.Info("sdtviewer: corpus changed", "paths", paths)
	s.broker.publish(paths)
}

// index returns the current search index (nil when unavailable).
func (s *server) index() *search.Index {
	s.srchMu.RLock()
	defer s.srchMu.RUnlock()
	return s.srch
}

// handleSearch serves ranked fulltext results from the in-memory bleve index:
// GET /api/search?q=&kind=&from=&to=&limit=. kind is an exact frontmatter kind
// filter; from/to bound the frontmatter created date (inclusive). Empty or
// missing q returns an empty result (never an error).
func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	max := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			max = n
		}
	}
	if s.index() == nil {
		writeJSON(w, http.StatusOK, search.Results{Results: []search.Result{}, Total: 0})
		return
	}
	res, err := s.index().Search(q, kind, from, to, max)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	if res.Results == nil {
		res.Results = []search.Result{}
	}
	writeJSON(w, http.StatusOK, res)
}
