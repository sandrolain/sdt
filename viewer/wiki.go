package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/graph"
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
	ID       string `json:"id"`
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Text     string `json:"text,omitempty"`
	Label    string `json:"label,omitempty"`
	XCluster string `json:"x-cluster,omitempty"`
}

type canvasEdge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromSide string `json:"fromSide"`
	ToNode   string `json:"toNode"`
	ToSide   string `json:"toSide"`
	Label    string `json:"label,omitempty"`
	XKind    string `json:"x-kind,omitempty"`
}

// board layout constants for the graph-derived canvas (cluster blocks).
const (
	boardNodeW     = 260
	boardNodeH     = 120
	boardNodeGapX  = 60
	boardNodeGapY  = 60
	boardBlockPad  = 40
	boardBlockGap  = 80
	boardBlockCols = 3
	boardTitleH    = 40
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
	// The edge list is derived once from the shared model and reused per
	// request by graph/rel/board (O2); the wiki is not reloaded on live-reload.
	s.wikiPageGraph = graph.From(nil, bld, nil)
	s.wikiEdgeList = s.computeWikiEdges()
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

// wikiEdges returns the edge list cached at wiki load (O2): graph/rel/board
// all read it, so the list is never recomputed per request.
func (s *server) wikiEdges() []graphEdge {
	return s.wikiEdgeList
}

// computeWikiEdges projects the shared edge model onto the wiki graph: the
// frontmatter relations become "relation" edges and the typed body links become
// "link" edges, with the page id as the node identity. Reading internal/graph
// rather than the pages means the graph, /api/wiki/rel and /api/backlinks cannot
// disagree about what is an edge.
func (s *server) computeWikiEdges() []graphEdge {
	var edges []graphEdge
	if s.wikiPageGraph == nil || s.wiki == nil {
		return edges
	}
	byRef := map[string]*contextwiki.Page{}
	for _, p := range s.wiki.Pages {
		byRef[graph.WikiRef(p)] = p
	}
	for _, e := range s.wikiPageGraph.Edges() {
		var kind string
		switch e.Family {
		case graph.Wiki:
			kind = "relation"
		case graph.Body:
			kind = "link"
		default:
			continue
		}
		src, tgt := byRef[e.From], byRef[e.To]
		if src == nil || tgt == nil {
			continue
		}
		edges = append(edges, graphEdge{
			Source: nodeID(src),
			Target: nodeID(tgt),
			Verb:   e.Verb,
			Label:  e.Label,
			Kind:   kind,
		})
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
	writeJSON(w, http.StatusOK, boardFromGraph(nodes, edges))
}

// boardFromGraph emits the wiki graph as JSON Canvas with graph parity (O3):
// one synthetic `group` node per cluster (the page `type`, sorted) enclosing
// exactly its pages, `x-cluster` on each page node, and the edge `kind` as
// `x-kind`. The layout is deterministic (sorted cluster ids, then page ids) so
// cluster blocks never overlap and the result is unit-testable. Unclustered
// pages (empty type) form a trailing block with no group node.
func boardFromGraph(nodes []graphNode, edges []graphEdge) boardResponse {
	byCluster := map[string][]graphNode{}
	var clusterIDs []string
	for _, n := range nodes {
		c := strings.TrimSpace(n.Type)
		if _, seen := byCluster[c]; !seen {
			clusterIDs = append(clusterIDs, c)
		}
		byCluster[c] = append(byCluster[c], n)
	}
	sort.Strings(clusterIDs)

	cn := make([]canvasNode, 0, len(nodes)+len(clusterIDs))
	y := 0
	for _, c := range clusterIDs {
		pages := byCluster[c]
		sort.Slice(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })
		cols := boardBlockCols
		if len(pages) < cols {
			cols = len(pages)
		}
		if cols < 1 {
			cols = 1
		}
		rows := (len(pages) + cols - 1) / cols
		blockW := boardBlockPad*2 + cols*boardNodeW + (cols-1)*boardNodeGapX
		blockH := boardTitleH + boardBlockPad + rows*boardNodeH + (rows-1)*boardNodeGapY + boardBlockPad
		if c != "" {
			cn = append(cn, canvasNode{
				ID: "cluster:" + c, Type: "group", Label: c,
				X: 0, Y: y, Width: blockW, Height: blockH,
			})
		}
		for i, p := range pages {
			col, row := i%cols, i/cols
			cn = append(cn, canvasNode{
				ID: p.ID, Type: "text", Text: p.Title, XCluster: c,
				X:      boardBlockPad + col*(boardNodeW+boardNodeGapX),
				Y:      y + boardTitleH + boardBlockPad + row*(boardNodeH+boardNodeGapY),
				Width:  boardNodeW,
				Height: boardNodeH,
			})
		}
		y += blockH + boardBlockGap
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
			XKind:    e.Kind,
		})
	}
	return boardResponse{Nodes: cn, Edges: ce}
}

// serveCanvasFile serves raw JSON Canvas content for a validated corpus path.
// Missing, traversal or non-<canvasExt> paths return 404.
func (s *server) serveCanvasFile(w http.ResponseWriter, rel string) {
	if filepath.Ext(rel) != canvasExt {
		writeErr(w, http.StatusNotFound, errNotFound)
		return
	}
	_, data, ok := s.readCorpusFile(w, rel)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, canvasResponse{Path: rel, Canvas: data})
}
