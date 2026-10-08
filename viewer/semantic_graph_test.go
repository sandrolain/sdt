package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sandrolain/sdt/internal/semantic"
)

// writeSemanticFixture writes a markdown fixture with the simple frontmatter the
// graph builder reads (kind/title/summary).
func writeSemanticFixture(t *testing.T, root, rel, kind, title string) {
	t.Helper()
	writeFixture(t, root, rel, "---\nkind: "+kind+"\ntitle: "+title+"\nsummary: "+title+" summary\n---\n\nbody\n")
}

// saveGraphSnapshot persists a hand-built vector snapshot (no encoder) so the
// endpoint tests stay offline-friendly. Version 1 is snapshotVersion in
// internal/semantic/snapshot.go; LoadSnapshot rejects any other value.
func saveGraphSnapshot(t *testing.T, root string, vectors map[string][]float32) {
	t.Helper()
	snap := &semantic.Snapshot{
		Version:        1,
		Model:          semantic.ModelBase8M,
		RecipeVersion:  semantic.RecipeVersion,
		SectionVectors: vectors,
	}
	if err := semantic.SaveSnapshot(root, snap); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
}

// graphGET requests /api/semantic/graph and decodes the payload.
func graphGET(t *testing.T, h http.Handler) semGraph {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/semantic/graph", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var g semGraph
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func edgeScore(g semGraph, from, to string) (float64, bool) {
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return e.Score, true
		}
	}
	return 0, false
}

// TestSemanticGraphScopedNodesAndEdges covers the eligible corpus scope and the
// de-duplicated top-k edge set: only knowledge kinds appear, an identical
// excluded record is dropped, and edges are ordered deterministically.
func TestSemanticGraphScopedNodesAndEdges(t *testing.T) {
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeSemanticFixture(t, root, "context/analysis/b.md", "analysis", "B")
	writeSemanticFixture(t, root, "context/architecture/c.md", "architecture", "C")
	writeSemanticFixture(t, root, "context/tasks/t.md", "tasks", "T")
	writeSemanticFixture(t, root, "context/wiki/w.md", "wiki", "W")
	saveGraphSnapshot(t, root, map[string][]float32{
		"context/notes/a.md#body":        {1, 0},
		"context/analysis/b.md#body":     {0.9, 0.1},
		"context/architecture/c.md#body": {0, 1},
		"context/tasks/t.md#body":        {1, 0},
		"context/wiki/w.md#body":         {1, 0},
	})

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	g := graphGET(t, s.mux())

	if len(g.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3 (a,b,c): %+v", len(g.Nodes), g.Nodes)
	}
	want := []string{"context/analysis/b.md", "context/architecture/c.md", "context/notes/a.md"}
	for i, n := range g.Nodes {
		if n.Path != want[i] {
			t.Errorf("node[%d] = %q, want %q", i, n.Path, want[i])
		}
		if n.Kind == "tasks" || n.Kind == "wiki" {
			t.Errorf("excluded kind in nodes: %+v", n)
		}
	}

	for _, e := range g.Edges {
		if e.From == "context/tasks/t.md" || e.To == "context/tasks/t.md" ||
			e.From == "context/wiki/w.md" || e.To == "context/wiki/w.md" {
			t.Errorf("excluded doc must not appear in edges: %+v", e)
		}
	}
	// Ordered by from then to.
	for i := 1; i < len(g.Edges); i++ {
		if g.Edges[i-1].From > g.Edges[i].From {
			t.Errorf("edges not sorted by from: %+v", g.Edges)
		}
	}
	if score, ok := edgeScore(g, "context/analysis/b.md", "context/notes/a.md"); !ok || score < 0.99 {
		t.Errorf("a-b edge = %v ok=%v, want ~0.994", score, ok)
	}
	if _, ok := edgeScore(g, "context/architecture/c.md", "context/notes/a.md"); ok {
		t.Error("orthogonal c-a pair must be dropped")
	}
}

// TestSemanticGraphAbsentIndexEmpty covers the degradation contract with
// on-demand building off: no snapshot yields an empty, error-free payload and no
// snapshot is created.
func TestSemanticGraphAbsentIndexEmpty(t *testing.T) {
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeSemanticFixture(t, root, "context/analysis/b.md", "analysis", "B")

	s, err := newServerWith(root, semanticOptions{BuildOnDemand: false}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := graphGET(t, s.mux())
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Errorf("absent index must yield an empty graph, got %+v", g)
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("opt-out graph endpoint must not create a snapshot")
	}
}

// TestSemanticGraphBuildsOnDemand covers the bootstrap contract: with an absent
// snapshot and building enabled, the endpoint builds it and then serves nodes.
func TestSemanticGraphBuildsOnDemand(t *testing.T) {
	probeViewerSemantic(t)
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeSemanticFixture(t, root, "context/analysis/b.md", "analysis", "B")
	writeFixture(t, root, ".sdt.yaml", "search:\n  model: BASE2M\n  semantic_build: true\n")

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	g := graphGET(t, s.mux())
	if len(g.Nodes) != 2 {
		t.Fatalf("on-demand build must serve 2 nodes, got %+v (warning=%q)", g, g.Warning)
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); err != nil {
		t.Fatalf("on-demand build must persist the snapshot: %v", err)
	}
}

// TestSemanticGraphBuildFailureWarns covers the failure degradation: an
// unbuildable model yields an empty graph with a warning, never an error, and no
// snapshot is written.
func TestSemanticGraphBuildFailureWarns(t *testing.T) {
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeFixture(t, root, ".sdt.yaml", "search:\n  model: NOPE\n  semantic_build: true\n")

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	g := graphGET(t, s.mux())
	if len(g.Nodes) != 0 {
		t.Errorf("failed build must stay empty, got %+v", g)
	}
	if g.Warning == "" {
		t.Error("failed build must carry a warning")
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("failed build must not write a snapshot")
	}
}

// TestSemanticGraphConcurrentBuild is a smoke test for the single-flight guard:
// concurrent requests all succeed and the snapshot ends consistent.
func TestSemanticGraphConcurrentBuild(t *testing.T) {
	probeViewerSemantic(t)
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeSemanticFixture(t, root, "context/analysis/b.md", "analysis", "B")
	writeFixture(t, root, ".sdt.yaml", "search:\n  model: BASE2M\n  semantic_build: true\n")

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	h := s.mux()
	const n = 4
	results := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/semantic/graph", nil))
			var g semGraph
			_ = json.Unmarshal(rec.Body.Bytes(), &g)
			results <- len(g.Nodes)
		}()
	}
	for i := 0; i < n; i++ {
		if got := <-results; got != 2 {
			t.Errorf("concurrent request nodes = %d, want 2", got)
		}
	}
}

// TestSemanticGraphBuildOptOut covers search.semantic_build=false: the endpoint
// stays read-only and creates no snapshot.
func TestSemanticGraphBuildOptOut(t *testing.T) {
	root := t.TempDir()
	writeSemanticFixture(t, root, "context/notes/a.md", "notes", "A")
	writeFixture(t, root, ".sdt.yaml", "search:\n  semantic_build: false\n")

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	g := graphGET(t, s.mux())
	if len(g.Nodes) != 0 {
		t.Errorf("opt-out must stay empty, got %+v", g)
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("opt-out must not create a snapshot")
	}
}
