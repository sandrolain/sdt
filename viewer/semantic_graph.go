package main

import (
	"net/http"
	"sort"

	"github.com/sandrolain/sdt/internal/semantic"
)

// semanticGraphK is the number of nearest scoped neighbours merged per node into
// the overlay edge set. It bounds the graph so the overlay stays readable.
const semanticGraphK = 5

// eligibleSemanticKinds is the knowledge-bearing scope of the docs semantic map.
// `wiki` is excluded because it has its own graph view (wiki and docs are
// separate uses); operational and generated records (tasks, plans, instructions,
// commands, worklog, ...) stay out so document structure does not dominate
// proximity.
var eligibleSemanticKinds = map[string]bool{
	"notes":        true,
	"analysis":     true,
	"decision":     true,
	"architecture": true,
	"research":     true,
}

// semNode is one document in the semantic-neighbour graph.
type semNode struct {
	Path    string `json:"path"`
	Kind    string `json:"kind,omitempty"`
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// semEdge is one undirected semantic-neighbour relation (from < to) shared by
// the graph overlay and the semantic tab.
type semEdge struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Score float64 `json:"score"`
}

// semGraph is the payload of /api/semantic/graph: the scoped nodes and their
// de-duplicated top-k neighbour edges, deterministically ordered. Warning is set
// when an on-demand build was attempted and failed, so the map can explain the
// empty state.
type semGraph struct {
	Nodes   []semNode `json:"nodes"`
	Edges   []semEdge `json:"edges"`
	Warning string    `json:"warning,omitempty"`
}

// handleSemanticGraph serves the scoped semantic-neighbour graph shared by the
// viewer's map view and semantic tab. When no snapshot exists and on-demand
// building is enabled it builds one first (single-flight), then reads it. It
// degrades to an empty graph with a warning when the build cannot run.
func (s *server) handleSemanticGraph(w http.ResponseWriter, r *http.Request) {
	warning := s.ensureSemanticSnapshot(r.Context())
	graph, err := s.buildSemanticGraph()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{Error: err.Error()})
		return
	}
	graph.Warning = warning
	writeJSON(w, http.StatusOK, graph)
}

// buildSemanticGraph computes the node and edge set over the eligible corpus
// scope, reusing the shipped document-vector aggregation and cosine (mean of
// section vectors; internal/semantic). An absent or empty snapshot yields an
// empty graph, never an error.
func (s *server) buildSemanticGraph() (semGraph, error) {
	entries, err := s.walkTree()
	if err != nil {
		return semGraph{}, err
	}
	meta := make(map[string]treeEntry, len(entries))
	for _, e := range entries {
		if eligibleSemanticKinds[e.Kind] {
			meta[e.Path] = e
		}
	}
	if len(meta) == 0 {
		return emptySemGraph(), nil
	}

	snap := semantic.LoadSnapshot(s.root)
	if snap.Empty() {
		return emptySemGraph(), nil
	}

	docs := snap.DocumentVectors()
	scoped := make(map[string][]float32, len(meta))
	for path := range meta {
		if v := docs[path]; len(v) > 0 {
			scoped[path] = v
		}
	}
	if len(scoped) == 0 {
		return emptySemGraph(), nil
	}

	paths := make([]string, 0, len(scoped))
	for p := range scoped {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	best := topKNeighbourEdges(paths, scoped)

	nodes := make([]semNode, 0, len(paths))
	for _, p := range paths {
		e := meta[p]
		nodes = append(nodes, semNode{Path: p, Kind: e.Kind, Title: e.Title, Summary: e.Summary})
	}

	edges := make([]semEdge, 0, len(best))
	for k, score := range best {
		edges = append(edges, semEdge{From: k[0], To: k[1], Score: score})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return semGraph{Nodes: nodes, Edges: edges}, nil
}

// topKNeighbourEdges returns the de-duplicated undirected edge set built from
// each scoped node's top-k nearest scoped neighbours. The pair key is ordered
// (a < b) and the higher score wins, so an edge appears once with a stable
// identity. Non-positive similarities are dropped.
func topKNeighbourEdges(paths []string, vecs map[string][]float32) map[[2]string]float64 {
	best := make(map[[2]string]float64)
	type neighbour struct {
		path  string
		score float64
	}
	for _, p := range paths {
		seed := vecs[p]
		nbrs := make([]neighbour, 0, len(paths)-1)
		for _, q := range paths {
			if q == p {
				continue
			}
			nbrs = append(nbrs, neighbour{path: q, score: semantic.Cosine(seed, vecs[q])})
		}
		sort.Slice(nbrs, func(i, j int) bool {
			if nbrs[i].score != nbrs[j].score {
				return nbrs[i].score > nbrs[j].score
			}
			return nbrs[i].path < nbrs[j].path
		})
		if len(nbrs) > semanticGraphK {
			nbrs = nbrs[:semanticGraphK]
		}
		for _, n := range nbrs {
			if n.score <= 0 {
				continue
			}
			a, b := p, n.path
			if a > b {
				a, b = b, a
			}
			key := [2]string{a, b}
			if cur, ok := best[key]; !ok || n.score > cur {
				best[key] = n.score
			}
		}
	}
	return best
}

// emptySemGraph is the degradation payload: valid JSON arrays, no findings.
func emptySemGraph() semGraph {
	return semGraph{Nodes: []semNode{}, Edges: []semEdge{}}
}
