package main

import (
	"net/http"
	"sort"
	"time"

	"github.com/sandrolain/sdt/internal/graph"
	"github.com/sandrolain/sdt/internal/mdindex"
)

// Backlinks: the inbound half of the corpus reference graph.
//
// The index is one projection of internal/graph, the model every surface reads:
// `sources` (derivation), `links` (correlation), the wiki frontmatter relations,
// typed body links and the lifecycle parent all arrive here as edges. The SPA
// never derives relations from /api/tree (an O(N) walk per render), and the
// wiki-neighbour endpoint reads the same model, so the two cannot disagree.

// backlinkDoc is one referring document in the /api/backlinks response.
type backlinkDoc struct {
	Path    string `json:"path"`
	Title   string `json:"title,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Summary string `json:"summary,omitempty"`
	// Via names the edge family that produced the reference: derivation,
	// correlation, wiki, body or lifecycle.
	Via string `json:"via"`
	// Verb is the relation verb when the family carries one (wiki, body).
	Verb string `json:"verb,omitempty"`
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

// familyRank orders the families when one document reaches a target through
// more than one: derivation outranks lifecycle (a plan that names its analysis
// in `sources` also records it as parent), and correlation is last, so a row
// states the strongest reason the edge exists rather than listing the same
// document twice.
func familyRank(f graph.Family) int {
	switch f {
	case graph.Derivation:
		return 0
	case graph.Lifecycle:
		return 1
	case graph.Wiki:
		return 2
	case graph.Body:
		return 3
	default: // graph.Correlation
		return 4
	}
}

// buildBacklinkIndexFrom projects an edge set onto "which documents point at
// this one". Metadata (title, kind, summary, modified) comes from the shared
// manifest; an edge whose source is not a manifest entry is skipped rather than
// listed as a bare path.
func buildBacklinkIndexFrom(m *mdindex.Manifest, g *graph.Graph) *backlinkIndex {
	index := &backlinkIndex{byTarget: map[string][]backlinkDoc{}}
	if m == nil || g == nil {
		return index
	}
	type pair struct{ target, from string }
	best := map[pair]graph.Edge{}
	for _, e := range g.Edges() {
		k := pair{target: e.To, from: e.From}
		cur, ok := best[k]
		if !ok || familyRank(e.Family) < familyRank(cur.Family) {
			best[k] = e
		}
	}
	for k, e := range best {
		src := m.Entries[k.from]
		if src == nil {
			continue
		}
		index.byTarget[k.target] = append(index.byTarget[k.target], backlinkDoc{
			Path:     k.from,
			Title:    src.Title,
			Kind:     src.Kind,
			Summary:  src.Summary,
			Via:      string(e.Family),
			Verb:     e.Verb,
			Modified: entryModified(src),
		})
	}
	index.sortAll()
	return index
}

// entryModified is the document's last modification as an RFC3339 string: the
// frontmatter `updated` when present, the file mtime otherwise.
func entryModified(e *mdindex.Entry) string {
	if e.Updated != "" {
		return e.Updated
	}
	if e.ModTimeNS != 0 {
		return time.Unix(0, e.ModTimeNS).UTC().Format(time.RFC3339)
	}
	return ""
}

// sortAll orders every referrer list newest first, path as the tiebreak.
func (b *backlinkIndex) sortAll() {
	for target, refs := range b.byTarget {
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].Modified != refs[j].Modified {
				return refs[i].Modified > refs[j].Modified
			}
			if refs[i].Path != refs[j].Path {
				return refs[i].Path < refs[j].Path
			}
			return refs[i].Via < refs[j].Via
		})
		b.byTarget[target] = refs
	}
}

// normalizeBacklinkRef maps a reference as written in frontmatter to the index
// key: a corpus-relative path (`context/analysis/x.md`) or a corpus-internal
// one (`analysis/x.md`), with `./` and a leading slash tolerated. Anything that
// does not point inside the corpus yields "".
func normalizeBacklinkRef(ref string) string {
	return graph.NormalizeRef(ref)
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
