package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backlinkCorpus builds a corpus exercising every edge source: a `sources`
// citation, a `links` correlation, a typed plan→analysis relation and a
// reference that points outside the served corpus.
func backlinkCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "context/analysis/analy-x.md", `---
kind: analysis
uid: 11111111-1111-4111-8111-111111111111
title: "Analysis X"
summary: "The analysis everything points at"
status: active
updated: "2026-09-20T10:00:00Z"
---

body
`)
	writeFixture(t, root, "context/plan/plan-x.md", `---
kind: plan
analysis_id: 11111111-1111-4111-8111-111111111111
title: "Plan X"
sources:
  - analysis/analy-x.md
---

plan body
`)
	writeFixture(t, root, "context/plan/plan-y.md", `---
kind: plan
title: "Plan Y"
links:
  - analysis/analy-x.md
---

plan y body
`)
	writeFixture(t, root, "context/tasks/plan-x-phase-1.md", `---
kind: tasks
plan_id: 22222222-2222-4222-8222-222222222222
title: "Plan X phase 1"
---

task body
`)
	writeFixture(t, root, "context/plan/plan-x-phase-1.md", `---
kind: plan
uid: 22222222-2222-4222-8222-222222222222
title: "Plan X phase 1"
---

phase body
`)
	writeFixture(t, root, "context/wiki/topic.map.md", `---
kind: wiki
title: "Orphan map"
links:
  - analysis/analy-x.md
---

map body
`)
	writeFixture(t, root, "context/notes/notes-z.md", `---
kind: notes
title: "Notes Z"
links:
  - wiki/topic.map.md
  - refs/clone.md
  - tmp/scratch.md
  - /outside.md
---

notes body
`)
	writeFixture(t, root, "context/refs/clone.md", "---\nkind: wiki\ntitle: Refs\n---\n")
	writeFixture(t, root, "context/tmp/scratch.md", "---\nkind: wiki\ntitle: Scratch\n---\n")
	return root
}

func backlinkRequest(t *testing.T, h http.Handler, path string) (int, backlinksResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/backlinks?path="+url.QueryEscape(path), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out backlinksResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode backlinks: %v (body %s)", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestBacklinksIndexRecordsEveryReferenceKind(t *testing.T) {
	index, err := buildBacklinkIndex(filepath.Join(backlinkCorpus(t), corpusDir))
	if err != nil {
		t.Fatal(err)
	}
	// the analysis is cited by two plans and one map, over two different lists
	got := index.Referrers("context/analysis/analy-x.md")
	if len(got) != 3 {
		t.Fatalf("analysis referrers = %d, want 3: %+v", len(got), got)
	}
	via := map[string]string{}
	for _, ref := range got {
		via[ref.Path] = ref.Via
	}
	if via["context/plan/plan-x.md"] != viaSources {
		t.Errorf("plan-x via = %q, want sources", via["context/plan/plan-x.md"])
	}
	if via["context/plan/plan-y.md"] != viaLinks {
		t.Errorf("plan-y via = %q, want links", via["context/plan/plan-y.md"])
	}
	if via["context/wiki/topic.map.md"] != viaLinks {
		t.Errorf("map via = %q, want links", via["context/wiki/topic.map.md"])
	}
	for _, ref := range got {
		if ref.Title == "" || ref.Kind == "" {
			t.Errorf("referrer missing metadata: %+v", ref)
		}
	}

	// the typed task->plan edge is recorded as a relation, not as a citation
	task := index.Referrers("context/plan/plan-x-phase-1.md")
	if len(task) != 1 || task[0].Path != "context/tasks/plan-x-phase-1.md" || task[0].Via != viaRelations {
		t.Errorf("plan-phase referrers = %+v, want one relation from the task file", task)
	}

	// one referrer citing a target twice records one edge: plan-x names the
	// analysis in `sources` and through `analysis_id`
	if n := countRef(index.Referrers("context/analysis/analy-x.md"), "context/plan/plan-x.md"); n != 1 {
		t.Errorf("plan-x recorded %d times", n)
	}
}

func TestBacklinksIndexDropsSelfReferences(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "context/notes/self.md", `---
kind: notes
links:
  - notes/self.md
---

body
`)
	index, err := buildBacklinkIndex(filepath.Join(root, corpusDir))
	if err != nil {
		t.Fatal(err)
	}
	if refs := index.Referrers("context/notes/self.md"); len(refs) != 0 {
		t.Errorf("self reference recorded: %+v", refs)
	}
}

func TestBacklinksIndexSkipsNonCorpusTargets(t *testing.T) {
	index, err := buildBacklinkIndex(filepath.Join(backlinkCorpus(t), corpusDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range index.Referrers("context/notes/notes-z.md") {
		switch ref.Via {
		case "links":
		default:
			t.Errorf("unexpected edge %+v", ref)
		}
	}
	// refs/ and tmp/ are outside the served corpus, so they are never targets
	for _, target := range []string{"context/refs/clone.md", "context/tmp/scratch.md"} {
		if refs := index.Referrers(target); len(refs) != 0 {
			t.Errorf("unexpected referrers for %s: %+v", target, refs)
		}
	}
}

func TestBacklinksIndexOrdersNewestFirst(t *testing.T) {
	index, err := buildBacklinkIndex(filepath.Join(backlinkCorpus(t), corpusDir))
	if err != nil {
		t.Fatal(err)
	}
	got := index.Referrers("context/analysis/analy-x.md")
	if len(got) < 2 {
		t.Fatalf("want several referrers, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Modified < got[i].Modified {
			t.Errorf("referrers not ordered newest first: %+v", got)
			break
		}
	}
}

func TestBacklinksIndexToleratesAMissingCorpus(t *testing.T) {
	dir := filepath.Join(t.TempDir(), corpusDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	index, err := buildBacklinkIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if refs := index.Referrers("context/analysis/analy-x.md"); len(refs) != 0 {
		t.Errorf("empty corpus returned %+v", refs)
	}
}

func TestNilBacklinkIndexIsSafe(t *testing.T) {
	var index *backlinkIndex
	if refs := index.Referrers("context/analysis/analy-x.md"); refs != nil {
		t.Errorf("nil index returned %+v", refs)
	}
}

func TestNormalizeBacklinkRef(t *testing.T) {
	cases := map[string]string{
		"context/analysis/analy-x.md":  "context/analysis/analy-x.md",
		"analysis/analy-x.md":          "context/analysis/analy-x.md",
		"./analysis/analy-x.md":        "context/analysis/analy-x.md",
		"/context/analysis/analy-x.md": "context/analysis/analy-x.md",
		"  analysis/analy-x.md  ":      "context/analysis/analy-x.md",
		"context/refs/clone.md":        "",
		"refs/clone.md":                "",
		"tmp/scratch.md":               "",
		"commands/ingest.md":           "context/commands/ingest.md",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeBacklinkRef(in); got != want {
			t.Errorf("normalizeBacklinkRef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandleBacklinksServesReferrers(t *testing.T) {
	h, err := newHandler(backlinkCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	code, out := backlinkRequest(t, h, "context/analysis/analy-x.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Total != len(out.Referrers) || out.Total != 3 {
		t.Errorf("total = %d, referrers = %d", out.Total, len(out.Referrers))
	}
	if out.Path != "context/analysis/analy-x.md" {
		t.Errorf("path = %q", out.Path)
	}
}

func TestHandleBacklinksUnknownTargetIsEmptyNot404(t *testing.T) {
	h, err := newHandler(backlinkCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	code, out := backlinkRequest(t, h, "context/wiki/orphan.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Total != 0 || out.Referrers == nil {
		t.Errorf("want an empty referrer list, got %+v", out)
	}
}

func TestHandleBacklinksRejectsPathsOutsideTheCorpus(t *testing.T) {
	h, err := newHandler(backlinkCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "docs/sdt_tokens.md", "/etc/passwd", "context/refs/clone.md", "../outside.md"} {
		code, _ := backlinkRequest(t, h, path)
		if code != http.StatusNotFound {
			t.Errorf("path %q status = %d, want 404", path, code)
		}
	}
}

func TestBacklinksSurviveAServerWithoutTheIndex(t *testing.T) {
	root := backlinkCorpus(t)
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	s.backlinks = nil
	req := httptest.NewRequest(http.MethodGet, "/api/backlinks?path=context/analysis/analy-x.md", nil)
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"referrers":[]`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func countRef(refs []backlinkDoc, path string) int {
	n := 0
	for _, ref := range refs {
		if ref.Path == path {
			n++
		}
	}
	return n
}
