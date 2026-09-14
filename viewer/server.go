package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/search"
)

const (
	// markdownExt and canvasExt are the served corpus extensions.
	markdownExt = ".md"
	canvasExt   = ".canvas"
	// corpusDir is the served knowledge base subdirectory under the project
	// root; everything outside it (refs/, docs/, root-level files) is private.
	corpusDir = "context"
	// errNotFound is the opaquely-shared 404 body across doc/wiki endpoints.
	errNotFound = "not found"
)

// indexHTML is a minimal placeholder served at / until the Phase 11 go:embed
// replaces it with the code-split web/ SPA.
const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>sdtviewer</title>
</head>
<body>
<h1>sdtviewer</h1>
<p>SPA placeholder — the web/ build is embedded in a later phase.</p>
</body>
</html>`

// server is the read-only httper of the corpus under root. corpus is the
// served subtree (root/context); everything else under root is out of scope.
type server struct {
	root   string
	corpus string
	wiki   *contextwiki.Builder
	srch   *search.Index
}

// treeEntry is one corpus file in the /api/tree listing.
type treeEntry struct {
	Path    string `json:"path"`
	Kind    string `json:"kind,omitempty"`
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Created string `json:"created,omitempty"`
	Canvas  bool   `json:"canvas,omitempty"`
}

// docResponse is the .md payload of /api/doc.
type docResponse struct {
	Path        string `json:"path"`
	Frontmatter string `json:"frontmatter"`
	Markdown    string `json:"markdown"`
}

// canvasResponse is the .canvas payload of /api/doc (raw JSON Canvas content).
type canvasResponse struct {
	Path   string          `json:"path"`
	Canvas json.RawMessage `json:"canvas"`
}

// errResponse mirrors errorResponse in sdt CLI responses across the API root.
type errResponse struct {
	Error string `json:"error"`
}

// newHandler builds the viewer mux for the given root directory.
func newHandler(root string) (http.Handler, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	s := &server{root: root, corpus: filepath.Join(root, corpusDir)}
	if err := s.loadWiki(); err != nil {
		log.Printf("sdtviewer: wiki graph unavailable: %v", err)
	}
	if err := s.loadSearch(); err != nil {
		log.Printf("sdtviewer: search unavailable: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/doc", s.handleDoc)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/wiki/graph", s.handleWikiGraph)
	mux.HandleFunc("/api/wiki/rel", s.handleWikiRel)
	mux.HandleFunc("/api/wiki/board", s.handleWikiBoard)
	mux.HandleFunc("/", s.handleIndex)
	return mux, nil
}

// handleIndex serves the SPA placeholder at / (and only at / in this phase).
func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(indexHTML)); err != nil {
		//nolint:errcheck -- best effort: client is gone
		_ = err
	}
}

// handleTree walks the corpus (root/context) excluding tmp/, scripts/ and
// refs/, returning .md (frontmatter title/summary/created) and .canvas files
// tagged with canvas. A missing corpus yields an empty listing.
func (s *server) handleTree(w http.ResponseWriter, _ *http.Request) {
	entries, err := s.walkTree()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// walkTree walks the corpus, skipping tmp/, scripts/ and refs/ directories
// (corpus noise), collecting .md entries and .canvas entries. Paths are
// project-root-relative (context/...), matching doc/search/wiki endpoints.
func (s *server) walkTree() ([]treeEntry, error) {
	info, err := os.Stat(s.corpus)
	if err != nil {
		if os.IsNotExist(err) {
			return []treeEntry{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", s.corpus)
	}
	var entries []treeEntry
	err = filepath.WalkDir(s.corpus, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == s.corpus {
				return nil
			}
			if d.Name() == "tmp" || d.Name() == "scripts" || d.Name() == "refs" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(s.corpus, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(filepath.Join(corpusDir, rel))
		switch filepath.Ext(rel) {
		case markdownExt:
			e, merr := s.mdEntry(path, rel)
			if merr != nil {
				return merr
			}
			entries = append(entries, e)
		case canvasExt:
			entries = append(entries, treeEntry{
				Path:   rel,
				Kind:   "canvas",
				Title:  strings.TrimSuffix(d.Name(), canvasExt),
				Canvas: true,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// mdEntry reads frontmatter fields (kind/title/summary/created) for one .md file.
func (s *server) mdEntry(path, rel string) (treeEntry, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk target
	if err != nil {
		return treeEntry{}, err
	}
	fm, _ := contextwiki.SplitFrontmatter(string(data))
	return treeEntry{
		Path:    rel,
		Kind:    contextwiki.FrontmatterField(fm, "kind"),
		Title:   contextwiki.FrontmatterField(fm, "title"),
		Summary: contextwiki.FrontmatterField(fm, "summary"),
		Created: contextwiki.FrontmatterField(fm, "created"),
	}, nil
}

// handleDoc serves a validated corpus file: frontmatter+markdown for .md, raw
// JSON Canvas content for .canvas. Missing/outside-corpus/unsupported = 404.
func (s *server) handleDoc(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, ok := s.safePath(rel)
	if !ok {
		writeJSON(w, http.StatusNotFound, errResponse{Error: errNotFound})
		return
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, errResponse{Error: errNotFound})
		return
	}
	data, err := os.ReadFile(full) //#nosec G304 -- path validated against corpus
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	switch filepath.Ext(full) {
	case markdownExt:
		fm, body := contextwiki.SplitFrontmatter(string(data))
		writeJSON(w, http.StatusOK, docResponse{Path: rel, Frontmatter: fm, Markdown: body})
	case canvasExt:
		writeJSON(w, http.StatusOK, canvasResponse{Path: rel, Canvas: json.RawMessage(data)})
	default:
		writeJSON(w, http.StatusNotFound, errResponse{Error: "unsupported file type"})
	}
}

// safePath resolves a project-root-relative corpus path (context/...) under
// the corpus, rejecting absolute, parent-escaped and non-corpus paths with
// filepath.Rel. ok=false for any invalid input.
func (s *server) safePath(rel string) (string, bool) {
	if rel == "" {
		return "", false
	}
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) {
		return "", false
	}
	slash := filepath.ToSlash(clean)
	if slash != corpusDir && !strings.HasPrefix(slash, corpusDir+"/") {
		return "", false
	}
	inner := strings.TrimPrefix(slash, corpusDir+"/")
	if inner == "" || inner == "." {
		return "", false
	}
	full := filepath.Join(s.corpus, filepath.FromSlash(inner))
	r, err := filepath.Rel(s.corpus, full)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.IsAbs(r) {
		return "", false
	}
	return full, true
}

// writeJSON writes v as JSON with the given status; a marshal error is
// non-recoverable and dumped to the client as an opaque 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("sdtviewer: json encode: %v", err)
	}
}
