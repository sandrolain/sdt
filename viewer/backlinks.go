package main

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/ctxrel"
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

// buildBacklinkIndex walks the corpus frontmatter once and records every
// outbound reference as an inbound edge on its target. A document that cannot
// be read is skipped: the corpus is not repaired here.
func buildBacklinkIndex(corpusDir string) (*backlinkIndex, error) {
	index := &backlinkIndex{byTarget: map[string][]backlinkDoc{}}
	edges, err := ctxrel.Load(corpusDir)
	if err != nil {
		return nil, err
	}
	walkErr := filepath.WalkDir(corpusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if path == corpusDir {
				return nil
			}
			if corpus.ExcludedDirName(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := backlinkRef(corpusDir, path)
		if rerr != nil || rel == "" {
			return rerr
		}
		if filepath.Ext(rel) != markdownExt || corpus.ExcludedPath(rel) {
			return nil
		}
		index.addDocument(rel, path, edges)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	index.sortAll()
	return index, nil
}

// addDocument records the references one document makes.
func (b *backlinkIndex) addDocument(rel, path string, edges *ctxrel.Edges) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk target
	if err != nil {
		return
	}
	fm, _ := contextwiki.SplitFrontmatter(string(data))
	kind := contextwiki.FrontmatterField(fm, "kind")
	referrer := backlinkDoc{
		Path:     rel,
		Title:    contextwiki.FrontmatterField(fm, "title"),
		Kind:     kind,
		Summary:  contextwiki.FrontmatterField(fm, "summary"),
		Modified: contextwiki.FrontmatterField(fm, "updated"),
	}
	if referrer.Modified == "" {
		if info, statErr := os.Stat(path); statErr == nil {
			referrer.Modified = info.ModTime().UTC().Format(time.RFC3339)
		}
	}
	seen := map[string]bool{}
	record := func(ref, via string) {
		target := normalizeBacklinkRef(ref)
		if target == "" || target == rel || seen[target] {
			return
		}
		seen[target] = true
		doc := referrer
		doc.Via = via
		b.byTarget[target] = append(b.byTarget[target], doc)
	}
	for _, ref := range contextwiki.FrontmatterList(fm, "sources") {
		record(ref, viaSources)
	}
	for _, ref := range contextwiki.FrontmatterList(fm, "links") {
		record(ref, viaLinks)
	}
	// The lifecycle parent is the typed relation, never a `sources` entry.
	switch kind {
	case "plan", "tasks":
		if parent := edges.ParentOf(rel); parent != "" {
			record(parent, viaRelations)
		}
	}
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

// backlinkRef converts a walked file path into the corpus-relative reference
// the index is keyed by (`context/analysis/x.md`).
func backlinkRef(corpusRoot, path string) (string, error) {
	rel, err := filepath.Rel(corpusRoot, path)
	if err != nil {
		return "", err
	}
	return corpusDir + "/" + filepath.ToSlash(rel), nil
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
