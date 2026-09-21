package main

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/search"
)

// loadSearch builds the shared section index over the markdown corpus at
// startup, backing it with the persistent store when available, and (when the
// opt-in semantic branch is enabled) the vector snapshot. A missing index (e.g.
// corpus-less root) is non-fatal: /api/search then degrades to an empty result.
func (s *server) loadSearch() error {
	refresh, err := mdindex.EnsureFresh(s.root)
	if err != nil {
		return err
	}
	ix, err := search.LoadOrRebuild(s.root, refresh.Manifest.EntriesSorted(), refresh.Changed, refresh.Removed)
	if err != nil {
		return err
	}
	s.srchMu.Lock()
	s.srch = ix
	s.srchMu.Unlock()
	s.rebuildSemantic(ix, refresh)
	return nil
}

// rebuildSearch rebuilds the index after a corpus change and publishes the
// changed paths to SSE subscribers. The store is updated incrementally from the
// manifest deltas; the semantic branch (when enabled) is refreshed from the
// snapshot; the old index is released.
func (s *server) rebuildSearch(paths []string) {
	refresh, err := mdindex.EnsureFresh(s.root)
	if err != nil {
		slog.Warn("sdtviewer: search rebuild scan failed", "err", err)
		return
	}
	ix, err := search.LoadOrRebuild(s.root, refresh.Manifest.EntriesSorted(), refresh.Changed, refresh.Removed)
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
	s.rebuildSemantic(ix, refresh)
	slog.Info("sdtviewer: corpus changed", "paths", paths)
	s.broker.publish(paths)
}

// index returns the current search index (nil when unavailable).
func (s *server) index() *search.Index {
	s.srchMu.RLock()
	defer s.srchMu.RUnlock()
	return s.srch
}

// handleSearch serves ranked results from the in-memory bleve index:
// GET /api/search?q=&kind=&objective=&from=&to=&limit=. kind is an exact
// frontmatter kind filter; objective is an exact frontmatter objective filter;
// from/to bound the frontmatter created date (inclusive). Empty or missing q
// returns an empty result (never an error). When the opt-in semantic branch is
// enabled at the server level, `semantic=1` returns RRF-fused hybrid results;
// filters apply to the lexical branch (semantic hits outside the filter set are
// dropped). Disabled or unavailable semantic never errors: it serves lexical.
func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	objective := r.URL.Query().Get("objective")
	status := r.URL.Query().Get("status")
	topic := r.URL.Query().Get("topic")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	max := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			max = n
		}
	}
	if s.index() == nil || q == "" {
		writeJSON(w, http.StatusOK, search.Results{Results: []search.Result{}, Total: 0})
		return
	}
	var res search.Results
	var err error
	if r.URL.Query().Get("semantic") == "1" {
		hq := search.HybridQuery{
			Q: q, Kind: kind, Objective: objective, Status: status,
			Topic: topic, From: from, To: to, Max: max,
		}
		res, err = s.index().SearchHybrid(r.Context(), hq, search.HybridOptions{Semantic: s.semanticIndex()})
	} else {
		res, err = s.index().Search(q, kind, objective, status, topic, from, to, max)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	if res.Results == nil {
		res.Results = []search.Result{}
	}
	writeJSON(w, http.StatusOK, res)
}
