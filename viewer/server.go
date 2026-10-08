package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/search"
	"github.com/sandrolain/sdt/internal/semantic"
)

const (
	// markdownExt, canvasExt and mermaidExt are the served corpus extensions.
	markdownExt = ".md"
	canvasExt   = ".canvas"
	mermaidExt  = ".mmd"
	// corpusDir is the served knowledge base subdirectory under the project
	// root; everything outside it (refs/, docs/, root-level files) is private.
	corpusDir = "context"
	// errNotFound is the opaquely-shared 404 body across doc/wiki endpoints.
	errNotFound = "not found"
)

// imageContentTypes allowlists the image extensions served by /api/file; any
// other extension is a 404 so the endpoint cannot expose arbitrary corpus files.
var imageContentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

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
	root       string
	corpus     string
	wiki       *contextwiki.Builder
	srch       *search.Index
	srchMu     sync.RWMutex
	backlinks  *backlinkIndex
	semOpts    semanticOptions
	sem        *semantic.Index
	semMu      sync.RWMutex
	semBuildMu sync.Mutex
	spa        http.Handler
	broker     *broker
	watcher    *fsnotify.Watcher
}

// treeEntry is one corpus file in the /api/tree listing.
//
// Analysis/Plans/Plan are the resolved lifecycle edges (a plan's single parent
// analysis, an analysis' plans, a task file's plan), taken from the typed
// `analysis_id`/`plan_id` relations by internal/ctxrel — the one resolver the Go
// tools share. A `sources` citation never becomes an edge here: Sources is the
// human-readable derivation list, displayed as a reference list, and the
// frontmatter `links` list is generic correlation and is deliberately absent.
// Surfaces that display the raw lists read them from the /api/doc frontmatter.
type treeEntry struct {
	Path       string   `json:"path"`
	Kind       string   `json:"kind,omitempty"`
	Title      string   `json:"title,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Objective  string   `json:"objective,omitempty"`
	Status     string   `json:"status,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Analysis   string   `json:"analysis,omitempty"`
	Plans      []string `json:"plans,omitempty"`
	Plan       string   `json:"plan,omitempty"`
	Sources    []string `json:"sources,omitempty"`
	Created    string   `json:"created,omitempty"`
	Modified   string   `json:"modified,omitempty"`
	Image      string   `json:"image,omitempty"`
	Canvas     bool     `json:"canvas,omitempty"`
	Mermaid    bool     `json:"mermaid,omitempty"`
	IsMap      bool     `json:"isMap,omitempty"`
	MapID      string   `json:"mapId,omitempty"`
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

// mermaidResponse is the .mmd payload of /api/doc (raw mermaid source).
type mermaidResponse struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// errResponse mirrors errorResponse in sdt CLI responses across the API root.
type errResponse struct {
	Error string `json:"error"`
}

// newHandler builds the viewer mux for the given root directory.
func newHandler(root string) (http.Handler, error) {
	s, err := newServer(root)
	if err != nil {
		return nil, err
	}
	return s.mux(), nil
}

// newServer stats the root and loads the in-memory caches. Semantic search is
// enabled from the root's .sdt.yaml (search.semantic/model), defaulting off.
func newServer(root string) (*server, error) {
	return newServerWith(root, semanticOptionsFromConfig(root), nil)
}

// newServerWith is newServer with explicit semantic options and optional flag
// overrides (precedence: override > options).
func newServerWith(root string, opts semanticOptions, overrides *semanticOverrides) (*server, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	if overrides != nil {
		overrides.apply(&opts)
	}
	s := &server{root: root, corpus: filepath.Join(root, corpusDir), broker: newBroker(), semOpts: opts}
	if h, ok := spaHandler(); ok {
		s.spa = h
	}
	if err := s.loadWiki(); err != nil {
		slog.Warn("sdtviewer: wiki graph unavailable", "err", err)
	}
	if err := s.loadSearch(); err != nil {
		slog.Warn("sdtviewer: search unavailable", "err", err)
	}
	index, err := buildBacklinkIndex(s.corpus)
	if err != nil {
		slog.Warn("sdtviewer: backlinks unavailable", "err", err)
	} else {
		s.backlinks = index
	}
	return s, nil
}

// mux registers the API routes on the server.
func (s *server) mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/doc", s.handleDoc)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/backlinks", s.handleBacklinks)
	mux.HandleFunc("/api/vocab", s.handleVocab)
	mux.HandleFunc("/api/semantic/graph", s.handleSemanticGraph)
	mux.HandleFunc("/api/wiki/graph", s.handleWikiGraph)
	mux.HandleFunc("/api/wiki/rel", s.handleWikiRel)
	mux.HandleFunc("/api/wiki/board", s.handleWikiBoard)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

// handleIndex serves the embedded SPA (with hash-route fallback) when a build
// is embedded, else the placeholder page at / only.
func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if s.spa != nil {
		s.spa.ServeHTTP(w, r)
		return
	}
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

// walkTree walks the corpus, skipping the corpus-excluded directories (tmp/,
// scripts/, refs/, commands/, instructions/, sdtdocs/) and the excluded
// context/README.md (corpus noise), collecting .md entries, .canvas entries and
// .mmd mermaid documents.
// Paths are project-root-relative (context/...), matching doc/search/wiki
// endpoints.
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
			if corpus.ExcludedDirName(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(s.corpus, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(filepath.Join(corpusDir, rel))
		if corpus.ExcludedPath(rel) {
			return nil
		}
		entry, ok, eerr := s.treeEntryFor(path, rel, d)
		if eerr != nil {
			return eerr
		}
		if ok {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	edges, eerr := ctxrel.Load(s.corpus)
	if eerr != nil {
		return nil, eerr
	}
	applyResolvedEdges(entries, edges)
	return entries, nil
}

// treeEntryFor builds the entry for one corpus file: markdown through the
// frontmatter reader, canvas and mermaid synthesized from the extension (they
// carry no frontmatter and therefore no lifecycle edge). ok is false for a file
// the tree does not list.
func (s *server) treeEntryFor(path, rel string, d fs.DirEntry) (treeEntry, bool, error) {
	var entry treeEntry
	switch filepath.Ext(rel) {
	case markdownExt:
		e, err := s.mdEntry(path, rel)
		if err != nil {
			return treeEntry{}, false, err
		}
		return e, true, nil
	case canvasExt:
		entry = treeEntry{
			Path:   rel,
			Kind:   "canvas",
			Title:  strings.TrimSuffix(d.Name(), canvasExt),
			Canvas: true,
		}
	case mermaidExt:
		entry = treeEntry{
			Path:    rel,
			Kind:    "mermaid",
			Title:   strings.TrimSuffix(d.Name(), mermaidExt),
			Mermaid: true,
		}
	default:
		return treeEntry{}, false, nil
	}
	if info, err := d.Info(); err == nil {
		entry.Modified = info.ModTime().UTC().Format(time.RFC3339)
	}
	return entry, true, nil
}

// applyResolvedEdges stamps the typed lifecycle edges onto the entries: a plan
// its single analysis, an analysis the plans that name it, a task file its plan.
// Non-markdown entries have no frontmatter and therefore no edge.
func applyResolvedEdges(entries []treeEntry, edges *ctxrel.Edges) {
	for i := range entries {
		ref := entries[i].Path
		switch entries[i].Kind {
		case "plan":
			entries[i].Analysis = edges.ParentOf(ref)
		case "analysis":
			entries[i].Plans = edges.ChildrenOf(ref)
		case "tasks":
			entries[i].Plan = edges.ParentOf(ref)
		}
	}
}

// mdEntry reads frontmatter fields (kind/title/summary/created) and the
// modified timestamp (frontmatter `updated`, else the file mtime) for one .md
// file. `created` falls back to the legacy `created_at` key so pre-rename
// documents still surface a creation date.
func (s *server) mdEntry(path, rel string) (treeEntry, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk target
	if err != nil {
		return treeEntry{}, err
	}
	fm, _ := contextwiki.SplitFrontmatter(string(data))
	// `sources` is the human-readable derivation list; `links` is correlation
	// and is never merged into it. The lifecycle edges the status dots read are
	// the resolved Analysis/Plans/Plan fields, stamped from the typed relations
	// by applyResolvedEdges after the walk.
	created := contextwiki.FrontmatterField(fm, "created")
	if created == "" {
		created = contextwiki.FrontmatterField(fm, "created_at")
	}
	e := treeEntry{
		Path:       rel,
		Kind:       contextwiki.FrontmatterField(fm, "kind"),
		Title:      contextwiki.FrontmatterField(fm, "title"),
		Summary:    contextwiki.FrontmatterField(fm, "summary"),
		Objective:  contextwiki.FrontmatterField(fm, "objective"),
		Status:     contextwiki.FrontmatterField(fm, "status"),
		Categories: contextwiki.FrontmatterList(fm, "categories"),
		Sources:    contextwiki.FrontmatterList(fm, "sources"),
		Created:    created,
		Modified:   contextwiki.FrontmatterField(fm, "updated"),
		Image:      contextwiki.FrontmatterField(fm, "image"),
	}
	if e.Modified == "" {
		if info, statErr := os.Stat(path); statErr == nil {
			e.Modified = info.ModTime().UTC().Format(time.RFC3339)
		}
	}
	if contextwiki.IsMapDoc(rel) {
		e.IsMap = true
		e.MapID = contextwiki.DocID(rel)
	}
	return e, nil
}

// handleDoc serves a validated corpus file: frontmatter+markdown for .md, raw
// JSON Canvas content for .canvas, raw mermaid source for .mmd.
// Missing/outside-corpus/unsupported = 404.
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
	case mermaidExt:
		writeJSON(w, http.StatusOK, mermaidResponse{Path: rel, Source: string(data)})
	default:
		writeJSON(w, http.StatusNotFound, errResponse{Error: "unsupported file type"})
	}
}

// handleFile serves an allowlisted image from the corpus (used for frontmatter
// `image:` thumbnails and inline body images). Non-images, missing files and
// paths outside the corpus are opaque 404s.
func (s *server) handleFile(w http.ResponseWriter, r *http.Request) {
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
	contentType, ok := imageContentTypes[strings.ToLower(filepath.Ext(full))]
	if !ok {
		writeJSON(w, http.StatusNotFound, errResponse{Error: errNotFound})
		return
	}
	data, err := os.ReadFile(full) //#nosec G304 -- path validated against corpus
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.Error("sdtviewer: write image", "err", err)
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
	if corpus.ExcludedPath(slash) {
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
		slog.Error("sdtviewer: json encode", "err", err)
	}
}
