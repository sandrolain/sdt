package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// wikiDir is the wiki corpus subdirectory under the served root.
const wikiDir = "context/wiki"

// graphNode is one wiki page node in /api/wiki/graph and the basis for the
// JSON Canvas board.
type graphNode struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Type    string   `json:"type,omitempty"`
	Status  string   `json:"status,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Path    string   `json:"path"`
}

// graphEdge is one directed relation in /api/wiki/graph: a frontmatter
// relation (kind "relation") or a typed body link (kind "link").
type graphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Verb   string `json:"verb"`
	Label  string `json:"label,omitempty"`
	Kind   string `json:"kind"`
}

// graphResponse is the /api/wiki/graph payload.
type graphResponse struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// relEntry is one directed neighbor of a page, grouped by verb.
type relEntry struct {
	Source  string `json:"source,omitempty"`
	Target  string `json:"target,omitempty"`
	Label   string `json:"label,omitempty"`
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Type    string `json:"type,omitempty"`
	Status  string `json:"status,omitempty"`
	Summary string `json:"summary,omitempty"`
	Path    string `json:"path"`
}

// relResponse is the /api/wiki/rel payload: inbound+outbound neighbors of one
// page grouped by verb, with direction/link-kind and target metadata.
type relResponse struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Inbound  map[string]relGroup `json:"inbound,omitempty"`
	Outbound map[string]relGroup `json:"outbound,omitempty"`
}

// relGroup is one verb bucket of rel entries, sorted by target/source.
type relGroup []relEntry

// boardResponse is the /api/wiki/board payload: the wiki graph emitted as
// JSON Canvas (v0.2) nodes/edges.
type boardResponse struct {
	Nodes []canvasNode `json:"nodes"`
	Edges []canvasEdge `json:"edges"`
}

// canvasNode / canvasEdge follow the JSON Canvas v0.2 shape.
type canvasNode struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Text   string `json:"text"`
}

type canvasEdge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromSide string `json:"fromSide"`
	ToNode   string `json:"toNode"`
	ToSide   string `json:"toSide"`
	Label    string `json:"label,omitempty"`
}

// board layout constants for the graph-derived canvas.
const (
	boardCols   = 4
	boardColGap = 320
	boardRowGap = 180
	boardWidth  = 260
	boardHeight = 120
)

// loadWiki loads the shared wiki Builder from <root>/context/wiki into the
// server. A missing wiki directory is not an error: the wiki endpoints then
// serve empty responses. Unresolvable/unreadable files are skipped by the
// builder (mirroring sdt context wiki lint).
func (s *server) loadWiki() error {
	dir := filepath.Join(s.root, wikiDir)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("wiki dir: %w", err)
	}
	bld, _, werr := contextwiki.LoadWiki(dir)
	if werr != nil {
		return werr
	}
	s.wiki = bld
	return nil
}

// handleWikiGraph serves nodes+edges from the shared builder. Relation
// frontmatter and typed body links become edges; .canvas files are never
// loaded into the builder, so they never appear here.
func (s *server) handleWikiGraph(w http.ResponseWriter, r *http.Request) {
	nodes, edges := s.wikiGraph()
	writeJSON(w, http.StatusOK, graphResponse{Nodes: nodes, Edges: edges})
}

// wikiGraph materializes the deterministic node/edge lists for the loaded
// builder. pages with no id use their file id as the node id.
func (s *server) wikiGraph() ([]graphNode, []graphEdge) {
	if s.wiki == nil {
		return []graphNode{}, []graphEdge{}
	}
	var nodes []graphNode
	for _, p := range s.wiki.Pages {
		nodes = append(nodes, graphNode{
			ID:      nodeID(p),
			Title:   p.Title,
			Type:    p.Type,
			Status:  p.Status,
			Tags:    p.Tags,
			Summary: p.Summary,
			Path:    filepath.ToSlash(filepath.Join(wikiDir, p.FileID+markdownExt)),
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	edges := s.wikiEdges()
	return nodes, edges
}

// nodeID is the graph identity of a page: the frontmatter id, falling back to
// the file id when absent.
func nodeID(p *contextwiki.Page) string {
	if p.ID != "" {
		return p.ID
	}
	return p.FileID
}

// wikiEdges builds the deduplicated, sorted edge list from relation
// frontmatter and typed body links. Edges keep the source verb; labels come
// from the wikilink label ("[[id|label]]") or the resolved target title. Only
// links with a verb (contextwiki typed links) become "link" edges; plain
// [[wiki]] mentions carry no relation.
func (s *server) wikiEdges() []graphEdge {
	var edges []graphEdge
	seen := map[string]bool{}
	add := func(e graphEdge) {
		key := e.Source + "\x00" + e.Verb + "\x00" + e.Target
		if !seen[key] {
			seen[key] = true
			edges = append(edges, e)
		}
	}
	for _, p := range s.wiki.Pages {
		s.addRelationEdges(p, add)
		s.addLinkEdges(p, add)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Verb != edges[j].Verb {
			return edges[i].Verb < edges[j].Verb
		}
		return edges[i].Target < edges[j].Target
	})
	return edges
}

// addRelationEdges emits "relation" edges from the frontmatter relations map.
func (s *server) addRelationEdges(p *contextwiki.Page, add func(graphEdge)) {
	src := nodeID(p)
	for verb, rawTargets := range p.Relations {
		for _, raw := range rawTargets {
			m := contextwiki.LinkRegexp.FindStringSubmatch(raw)
			if m == nil {
				continue
			}
			tgt := s.wiki.Resolve(strings.TrimSpace(m[1]))
			if tgt == nil {
				continue
			}
			label := strings.TrimSpace(m[2])
			if label == "" {
				label = tgt.Title
			}
			add(graphEdge{Source: src, Target: nodeID(tgt), Verb: verb, Label: label, Kind: "relation"})
		}
	}
}

// addLinkEdges emits "link" edges for typed body links (verb != "").
func (s *server) addLinkEdges(p *contextwiki.Page, add func(graphEdge)) {
	src := nodeID(p)
	for _, l := range p.Links {
		if l.Verb == "" {
			continue
		}
		tgt := s.wiki.Resolve(l.Target)
		if tgt == nil {
			continue
		}
		add(graphEdge{Source: src, Target: nodeID(tgt), Verb: l.Verb, Label: tgt.Title, Kind: "link"})
	}
}

// handleWikiRel serves inbound+outbound neighbors of one page, grouped by
// verb. The id is resolved via the shared builder (id or unique title).
func (s *server) handleWikiRel(w http.ResponseWriter, r *http.Request) {
	if s.wiki == nil {
		writeJSON(w, http.StatusNotFound, errResponse{Error: "wiki not loaded"})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errResponse{Error: "missing id"})
		return
	}
	p := s.wiki.Resolve(id)
	if p == nil {
		writeJSON(w, http.StatusNotFound, errResponse{Error: "page not found"})
		return
	}
	byID := map[string]*contextwiki.Page{}
	for _, q := range s.wiki.Pages {
		byID[nodeID(q)] = q
	}
	self := nodeID(p)
	resp := relResponse{ID: self, Title: p.Title, Inbound: map[string]relGroup{}, Outbound: map[string]relGroup{}}
	for _, e := range s.wikiEdges() {
		if e.Source == self {
			tgt := byID[e.Target]
			resp.Outbound[e.Verb] = append(resp.Outbound[e.Verb], relEntry{
				Target:  nodeID(tgt),
				Label:   e.Label,
				Kind:    e.Kind,
				Title:   tgt.Title,
				Type:    tgt.Type,
				Status:  tgt.Status,
				Summary: tgt.Summary,
				Path:    filepath.ToSlash(filepath.Join(wikiDir, tgt.FileID+markdownExt)),
			})
		}
		if e.Target == self {
			src := byID[e.Source]
			resp.Inbound[e.Verb] = append(resp.Inbound[e.Verb], relEntry{
				Source:  nodeID(src),
				Label:   e.Label,
				Kind:    e.Kind,
				Title:   src.Title,
				Type:    src.Type,
				Status:  src.Status,
				Summary: src.Summary,
				Path:    filepath.ToSlash(filepath.Join(wikiDir, src.FileID+markdownExt)),
			})
		}
	}
	for _, group := range resp.Outbound {
		sort.Slice(group, func(i, j int) bool { return group[i].Target < group[j].Target })
	}
	for _, group := range resp.Inbound {
		sort.Slice(group, func(i, j int) bool { return group[i].Source < group[j].Source })
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleWikiBoard serves the graph as JSON Canvas, or a selected .canvas file
// when ?file=<rel> is requested.
func (s *server) handleWikiBoard(w http.ResponseWriter, r *http.Request) {
	if file := r.URL.Query().Get("file"); file != "" {
		s.serveCanvasFile(w, file)
		return
	}
	nodes, edges := s.wikiGraph()
	cn := make([]canvasNode, 0, len(nodes))
	for i, n := range nodes {
		cn = append(cn, canvasNode{
			ID:     n.ID,
			Type:   "text",
			X:      (i % boardCols) * boardColGap,
			Y:      (i / boardCols) * boardRowGap,
			Width:  boardWidth,
			Height: boardHeight,
			Text:   n.Title,
		})
	}
	ce := make([]canvasEdge, 0, len(edges))
	for i, e := range edges {
		ce = append(ce, canvasEdge{
			ID:       fmt.Sprintf("e%d", i),
			FromNode: e.Source,
			FromSide: "right",
			ToNode:   e.Target,
			ToSide:   "left",
			Label:    e.Verb,
		})
	}
	writeJSON(w, http.StatusOK, boardResponse{Nodes: cn, Edges: ce})
}

// serveCanvasFile serves raw JSON Canvas content for a validated corpus path.
// Missing, traversal or non-<canvasExt> paths return 404.
func (s *server) serveCanvasFile(w http.ResponseWriter, rel string) {
	full, ok := s.safePath(rel)
	if !ok || filepath.Ext(full) != canvasExt {
		writeJSON(w, http.StatusNotFound, errResponse{Error: "not found"})
		return
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, errResponse{Error: "not found"})
		return
	}
	data, err := os.ReadFile(full) //#nosec G304 -- path validated in safePath
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, canvasResponse{Path: rel, Canvas: data})
}
