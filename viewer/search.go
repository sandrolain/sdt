package main

import (
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
	s.srch = ix
	return nil
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
	if s.srch == nil {
		writeJSON(w, http.StatusOK, search.Results{Results: []search.Result{}, Total: 0})
		return
	}
	res, err := s.srch.Search(q, kind, from, to, max)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	if res.Results == nil {
		res.Results = []search.Result{}
	}
	writeJSON(w, http.StatusOK, res)
}
