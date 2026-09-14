package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// makeWikiCorpus builds a corpus with relation and typed-link bearing wiki
// pages, an archived page, and a .canvas file that must stay out of the graph.
func makeWikiCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "context/wiki/alpha.md", `---
kind: wiki
id: alpha
title: Alpha
type: module
status: active
summary: The alpha module
relations:
  depends_on:
    - "[[beta|Beta module]]"
  part_of:
    - "[[gamma|Gamma]]"
tags:
  - core
---

Body with [[supersedes::epsilon|Epsilon]] link.
`)
	writeFixture(t, root, "context/wiki/beta.md", `---
kind: wiki
id: beta
title: Beta
type: module
status: active
summary: The beta module
---

beta body
`)
	writeFixture(t, root, "context/wiki/gamma.md", `---
kind: wiki
id: gamma
title: Gamma
type: entity
status: archived
summary: Old gamma
---

gamma body
`)
	writeFixture(t, root, "context/wiki/epsilon.md", `---
kind: wiki
id: epsilon
title: Epsilon
type: module
status: draft
---

epsilon body
`)
	writeFixture(t, root, "context/wiki/noid.md", `---
kind: wiki
title: No id page
type: module
status: draft
---

no id body
`)
	writeFixture(t, root, "context/wiki/board.canvas", `{"nodes":[{"id":"a"}],"edges":[]}`)
	return root
}

func TestWikiGraph(t *testing.T) {
	root := makeWikiCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/wiki/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 5 {
		t.Errorf("nodes = %d, want 5: %+v", len(out.Nodes), out.Nodes)
	}
	nodesByID := map[string]graphNode{}
	for _, n := range out.Nodes {
		nodesByID[n.ID] = n
	}
	if n, ok := nodesByID["alpha"]; !ok {
		t.Error("alpha node missing")
	} else if n.Title != "Alpha" || n.Type != "module" || n.Status != "active" || n.Summary != "The alpha module" {
		t.Errorf("alpha node wrong: %+v", n)
	} else if n.Path != "context/wiki/alpha.md" {
		t.Errorf("alpha path = %q", n.Path)
	}
	// page without frontmatter id falls back to file id
	if _, ok := nodesByID["noid"]; !ok {
		t.Error("noid node missing (file id fallback)")
	}
	// archive/entity types preserved
	if n, ok := nodesByID["gamma"]; !ok || n.Type != "entity" || n.Status != "archived" {
		t.Errorf("gamma node wrong: %+v", n)
	}
	// edges: 2 relations from alpha + 1 typed link from alpha
	if len(out.Edges) != 3 {
		t.Fatalf("edges = %d, want 3: %+v", len(out.Edges), out.Edges)
	}
	bySource := map[string][]graphEdge{}
	for _, e := range out.Edges {
		bySource[e.Source] = append(bySource[e.Source], e)
	}
	alphaEdges := bySource["alpha"]
	if len(alphaEdges) != 3 {
		t.Fatalf("alpha edges = %d, want 3: %+v", len(alphaEdges), alphaEdges)
	}
	seenKind := map[string]bool{}
	for _, e := range alphaEdges {
		seenKind[e.Kind] = true
		// every edge carries a verb and a label
		if e.Verb == "" || e.Label == "" {
			t.Errorf("edge missing verb/label: %+v", e)
		}
	}
	if !seenKind["relation"] || !seenKind["link"] {
		t.Errorf("expected relation+link kinds, got %v", seenKind)
	}
}

func TestWikiGraphCanvasExcluded(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/graph", nil))
	var out graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, n := range out.Nodes {
		if strings.Contains(n.ID, "board") || n.Path == "context/wiki/board.canvas" {
			t.Errorf("canvas leaked into nodes: %+v", n)
		}
	}
	for _, e := range out.Edges {
		if strings.Contains(e.Source, "board") || strings.Contains(e.Target, "board") {
			t.Errorf("canvas leaked into edges: %+v", e)
		}
	}
}

func TestWikiGraphEmpty(t *testing.T) {
	h, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/graph", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 0 || len(out.Edges) != 0 {
		t.Errorf("expected empty graph, got %+v", out)
	}
}

func TestWikiRelOutbound(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/wiki/rel?id="+url.QueryEscape("alpha"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out relResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "alpha" || out.Title != "Alpha" {
		t.Errorf("rel head = %+v", out)
	}
	// outbound grouped by verb
	dep := out.Outbound["depends_on"]
	if len(dep) != 1 {
		t.Fatalf("depends_on = %+v", dep)
	}
	if dep[0].Target != "beta" || dep[0].Title != "Beta" || dep[0].Path != "context/wiki/beta.md" || dep[0].Kind != "relation" {
		t.Errorf("depends_on entry = %+v", dep[0])
	}
	// typed body link surfaces as an outbound link edge with the verb label
	sup := out.Outbound["supersedes"]
	if len(sup) != 1 || sup[0].Kind != "link" || sup[0].Target != "epsilon" || sup[0].Label != "Epsilon" {
		t.Errorf("supersedes (link) = %+v", sup)
	}
	// inbound from gamma? no — gamma has no inbound. instead verify inbound of
	// beta contains alpha via depends_on.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/wiki/rel?id=beta", nil))
	var beta relResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &beta); err != nil {
		t.Fatal(err)
	}
	depIn := beta.Inbound["depends_on"]
	if len(depIn) != 1 || depIn[0].Source != "alpha" || depIn[0].Label != "Beta module" {
		t.Errorf("beta inbound depends_on = %+v", depIn)
	}
}

func TestWikiRelInboundMetadata(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/rel?id=gamma", nil))
	var out relResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	po := out.Inbound["part_of"]
	if len(po) != 1 {
		t.Fatalf("inbound part_of = %+v", po)
	}
	if po[0].Source != "alpha" || po[0].Title != "Alpha" || po[0].Path != "context/wiki/alpha.md" || po[0].Type != "module" || po[0].Status != "active" {
		t.Errorf("inbound entry = %+v", po[0])
	}
}

func TestWikiRelMissingID(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/rel", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWikiRelNotFound(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/rel?id=ghost", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWikiRelEmptyWiki(t *testing.T) {
	h, _ := newHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/rel?id=alpha", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWikiBoardDefault(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/board", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out boardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 5 {
		t.Errorf("board nodes = %d", len(out.Nodes))
	}
	if out.Nodes[0].Type != "text" || out.Nodes[0].X != 0 || out.Nodes[0].Y != 0 || out.Nodes[0].Text == "" {
		t.Errorf("board node[0] = %+v", out.Nodes[0])
	}
	if len(out.Edges) != 3 {
		t.Errorf("board edges = %d", len(out.Edges))
	}
	if out.Edges[0].FromNode == "" || out.Edges[0].ToNode == "" {
		t.Errorf("board edge[0] = %+v", out.Edges[0])
	}
}

func TestWikiBoardCanvasFile(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/api/wiki/board?file="+url.QueryEscape("context/wiki/board.canvas"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out canvasResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Path != "context/wiki/board.canvas" {
		t.Errorf("path = %q", out.Path)
	}
	var raw map[string]any
	if err := json.Unmarshal(out.Canvas, &raw); err != nil {
		t.Fatalf("canvas not raw: %v", err)
	}
	if raw["nodes"] == nil {
		t.Error("canvas nodes missing")
	}
}

func TestWikiBoardCanvasTraversal(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	for _, path := range []string{"../board.canvas", "context/tmp/board.canvas", "/etc/passwd"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/board?file="+url.QueryEscape(path), nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d", path, rec.Code)
		}
	}
}

func TestWikiBoardMissingCanvas(t *testing.T) {
	root := makeWikiCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/board?file=context/wiki/none.canvas", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWikiBoardEmptyWiki(t *testing.T) {
	h, _ := newHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/wiki/board", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out boardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 0 || len(out.Edges) != 0 {
		t.Errorf("expected empty board, got %+v", out)
	}
}

func TestLoadWikiMissingDir(t *testing.T) {
	s := &server{root: t.TempDir()}
	if err := s.loadWiki(); err == nil {
		t.Error("expected error for missing wiki dir")
	}
	if s.wiki != nil {
		t.Error("wiki should stay nil on missing dir")
	}
}
