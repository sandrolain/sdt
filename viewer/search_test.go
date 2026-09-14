package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sandrolain/sdt/internal/search"
)

func TestSearchHandlerRanked(t *testing.T) {
	root := makeSearchCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape("tokens"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 {
		t.Errorf("total = %d, want 3", out.Total)
	}
	has := map[string]bool{}
	for _, r := range out.Results {
		has[r.Path] = true
		if r.Snippet == "" {
			t.Errorf("empty snippet for %s", r.Path)
		}
	}
	for _, p := range []string{"context/wiki/model.md", "context/wiki/api.md", "context/notes/note.md"} {
		if !has[p] {
			t.Errorf("missing %s in %v", p, has)
		}
	}
}

func TestSearchHandlerFilters(t *testing.T) {
	root := makeSearchCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?q=tokens&kind=wiki", nil))
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Errorf("kind=wiki total = %d, want 2", out.Total)
	}
	for _, r := range out.Results {
		if r.Kind != "wiki" {
			t.Errorf("non-wiki result: %+v", r)
		}
	}
	// date bounds
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/search?q=tokens&from=2026-09-11&to=2026-09-12", nil))
	out2 := search.Results{}
	if err := json.Unmarshal(rec2.Body.Bytes(), &out2); err != nil {
		t.Fatal(err)
	}
	for _, r := range out2.Results {
		switch r.Path {
		case "context/wiki/api.md", "context/notes/note.md":
		default:
			t.Errorf("out-of-range result: %+v", r)
		}
	}
}

func TestSearchHandlerEmptyQ(t *testing.T) {
	root := makeSearchCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?q=", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 0 || out.Total != 0 {
		t.Errorf("empty q: %+v", out)
	}
}

func TestSearchHandlerNoIndex(t *testing.T) {
	// corpus-less root: index nil → empty result, still 200
	h, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?q=tokens", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 0 {
		t.Errorf("no-index results: %+v", out.Results)
	}
}

func TestSearchHandlerLimit(t *testing.T) {
	root := makeSearchCorpus(t)
	h, _ := newHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?q=tokens&limit=1", nil))
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 1 {
		t.Errorf("limit=1 returned %d", len(out.Results))
	}
}

// makeSearchCorpus builds a corpus with a wiki (2 pages), an analysis and a
// note, all mentioning the term "tokens".
func makeSearchCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "context/wiki/model.md", `---
kind: wiki
title: Model page
summary: internal tokens model
created: 2026-09-10
---

model body
`)
	writeFixture(t, root, "context/wiki/api.md", `---
kind: wiki
title: Api page
summary: api surface
created: 2026-09-12
---

api body about tokens.
`)
	writeFixture(t, root, "context/analysis/ana.md", `---
kind: analysis
title: Analysis
summary: neutral wording
created: 2026-09-11
---

no matches here
`)
	writeFixture(t, root, "context/notes/note.md", `---
kind: notes
title: Note
created: 2026-09-11
---

a note page mentioning tokens.
`)
	writeFixture(t, root, "context/tmp/scratch.md", `---
kind: wiki
title: Scratch
created: 2026-09-01
---

scratch tokens
`)
	writeFixture(t, root, "context/scripts/x.md", `---
kind: wiki
title: Script
created: 2026-09-01
---

script tokens
`)
	return root
}
