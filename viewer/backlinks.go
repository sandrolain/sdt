package main

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/mdindex"
)

// Backlinks: the inbound half of the corpus reference graph.
//
// `sources` is the human-readable derivation list, `links` the generic
// correlation list, and the lifecycle edges (a plan's analysis, a task file's
// plan) come from internal/ctxrel — the one resolver every Go tool shares. The
// index is built once at startup over the corpus frontmatter and answers
// "which documents point at this one", so the SPA never derives relations from
// /api/tree (an O(N) walk over ~800 documents per render).

// The three ways a document can point at another. `via` on a backlink tells
// the UI why the edge exists.
const (
	viaSources   = "sources"
	viaLinks     = "links"
	viaRelations = "relation"
)

// backlinkDoc is one referring document in the /api/backlinks response.
type backlinkDoc struct {
	Path    string `json:"path"`
	Title   string `json:"title,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Summary string `json:"summary,omitempty"`
	// Via names the frontmatter list (or the resolved relation) that produced
	// the edge.
	Via string `json:"via"`
	// Modified orders the list newest first; empty when unknown.
	Modified string `json:"modified,omitempty"`
}

// backlinksResponse is the /api/backlinks payload for one target.
type backlinksResponse struct {
	Path      string        `json:"path"`
	Total     int           `json:"total"`
	Referrers []backlinkDoc `json:"referrers"`
}

// backlinkIndex maps a target reference to the documents referencing it.
type backlinkIndex struct {
	byTarget map[string][]backlinkDoc
}

// Referrers returns the documents pointing at ref, newest first, for a
// reference the index does not know.
func (b *backlinkIndex) Referrers(ref string) []backlinkDoc {
	if b == nil {
		return nil
	}
	return b.byTarget[normalizeBacklinkRef(ref)]
}

// buildBacklinkIndex builds the inbound reference index from the shared
// manifest (O1), reusing the same entries as /api/tree instead of re-walking
// and re-parsing the corpus. It scans the corpus's parent root so the manifest
// covers root/context.
func buildBacklinkIndex(corpusDir string) (*backlinkIndex, error) {
	refresh, err := mdindex.EnsureFresh(filepath.Dir(corpusDir))
	if err != nil {
		return nil, err
	}
	edges, err := ctxrel.Load(corpusDir)
	if err != nil {
		return nil, err
	}
	return buildBacklinkIndexFromManifest(refresh.Manifest, edges), nil
}

// sortAll orders every referrer list newest first, path as the tiebreak.
func (b *backlinkIndex) sortAll() {
	for target, refs := range b.byTarget {
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].Modified != refs[j].Modified {
				return refs[i].Modified > refs[j].Modified
			}
			return refs[i].Path < refs[j].Path
		})
		b.byTarget[target] = refs
	}
}

// normalizeBacklinkRef maps a reference as written in frontmatter to the index
// key: a corpus-relative path (`context/analysis/x.md`) or a corpus-internal
// one (`analysis/x.md`), with `./` and a leading slash tolerated. Anything that
// does not point inside the corpus yields "".
func normalizeBacklinkRef(ref string) string {
	p := strings.TrimSpace(filepath.ToSlash(ref))
	if p == "" {
		return ""
	}
	p = strings.TrimPrefix(strings.TrimPrefix(p, "./"), "/")
	if !strings.HasPrefix(p, corpusDir+"/") {
		// `sources`/`links` are written corpus-internal (`analysis/x.md`), so a
		// bare reference is read as one; refs/ is outside the served corpus and
		// no viewer document has it.
		if strings.HasPrefix(p, "refs/") {
			return ""
		}
		p = corpusDir + "/" + p
	}
	if corpus.ExcludedPath(p) {
		return ""
	}
	return p
}

// handleBacklinks serves GET /api/backlinks?path=<corpus path>: the documents
// referencing the target, newest first. A path outside the corpus (or absent)
// is a 404, like the other doc endpoints.
func (s *server) handleBacklinks(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if _, ok := s.safePath(rel); !ok {
		writeJSON(w, http.StatusNotFound, errResponse{Error: errNotFound})
		return
	}
	referrers := s.backlinks.Referrers(rel)
	if referrers == nil {
		referrers = []backlinkDoc{}
	}
	writeJSON(w, http.StatusOK, backlinksResponse{
		Path:      rel,
		Total:     len(referrers),
		Referrers: referrers,
	})
}
