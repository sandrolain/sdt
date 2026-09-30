package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/sandrolain/sdt/internal/search"
)

// searchResultsAlias keeps the test readable without shadowing the search
// package name.
type searchResultsAlias = search.Results

// writeSearchDoc writes one markdown corpus document with a known timestamp.
func writeSearchDoc(t *testing.T, root, rel, updated string) {
	t.Helper()
	writeFixture(t, root, rel, "---\nkind: notes\ntitle: \"Doc "+rel+"\"\nupdated: \""+updated+"\"\n---\n\nbody\n")
}

func searchGet(t *testing.T, h http.Handler, rawQuery string) (int, searchResultsAlias) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/search?"+rawQuery, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out searchResultsAlias
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode search: %v (body %s)", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestSearchBrowsesWithAnEmptyQuery(t *testing.T) {
	root := t.TempDir()
	writeSearchDoc(t, root, "context/notes/older.md", "2026-09-01T00:00:00Z")
	writeSearchDoc(t, root, "context/notes/newer.md", "2026-09-20T00:00:00Z")
	writeSearchDoc(t, root, "context/notes/newest.md", "2026-09-29T00:00:00Z")
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	code, out := searchGet(t, h, "limit=20")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Total != 3 || len(out.Results) != 3 {
		t.Fatalf("total = %d, results = %d", out.Total, len(out.Results))
	}
	want := []string{"context/notes/newest.md", "context/notes/newer.md", "context/notes/older.md"}
	for i, path := range want {
		if out.Results[i].Path != path {
			t.Errorf("result %d = %q, want %q", i, out.Results[i].Path, path)
		}
	}
}

func TestSearchBrowseAppliesFiltersAndPages(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "context/plan/p1.md", "---\nkind: plan\ntitle: P1\nstatus: active\nupdated: \"2026-09-10T00:00:00Z\"\n---\n\nbody\n")
	writeFixture(t, root, "context/plan/p2.md", "---\nkind: plan\ntitle: P2\nstatus: completed\nupdated: \"2026-09-20T00:00:00Z\"\n---\n\nbody\n")
	writeFixture(t, root, "context/notes/n1.md", "---\nkind: notes\ntitle: N1\nupdated: \"2026-09-25T00:00:00Z\"\n---\n\nbody\n")
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	// kind filter over a browse
	_, out := searchGet(t, h, "kind=plan&limit=20")
	if out.Total != 2 || len(out.Results) != 2 {
		t.Fatalf("kind filter: total = %d, results = %d", out.Total, len(out.Results))
	}
	// status filter, and the true total is reported beyond the page
	_, out = searchGet(t, h, "status=active&limit=1")
	if out.Total != 1 || len(out.Results) != 1 {
		t.Errorf("status filter: total = %d, results = %d", out.Total, len(out.Results))
	}
	_, out = searchGet(t, h, "kind=plan&limit=1")
	if out.Total != 2 || len(out.Results) != 1 {
		t.Errorf("paging: total = %d, results = %d", out.Total, len(out.Results))
	}
	if out.Results[0].Path != "context/plan/p2.md" {
		t.Errorf("first browse hit = %q, want the newest plan", out.Results[0].Path)
	}
}

func TestSearchBrowseWithNoIndexIsEmptyNotAnError(t *testing.T) {
	root := t.TempDir() // no context/ dir at all
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	code, out := searchGet(t, h, "")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Total != 0 || out.Results == nil {
		t.Errorf("want an empty browse, got %+v", out)
	}
}

func TestSearchPopulatesTheMatchedSection(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "context/notes/n1.md", "---\nkind: notes\ntitle: Notes one\n---\n\n# Notes one\n\nintro text\n\n## Findings\n\nthe token lives here\n")
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	code, out := searchGet(t, h, "q=token")
	if code != http.StatusOK || len(out.Results) == 0 {
		t.Fatalf("status = %d, results = %d", code, len(out.Results))
	}
	if out.Results[0].Section != "findings" {
		t.Errorf("section = %q, want findings", out.Results[0].Section)
	}
	// a term that only appears in the summary is not attributed to a section
	_, out = searchGet(t, h, "q=intro")
	if len(out.Results) == 0 || out.Results[0].Section == "" {
		t.Errorf("expected a section for a body hit, got %+v", out.Results)
	}
}

func TestSearchSectionIsEmptyForBrowseHits(t *testing.T) {
	root := t.TempDir()
	writeSearchDoc(t, root, "context/notes/n1.md", "2026-09-10T00:00:00Z")
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	_, out := searchGet(t, h, "")
	if len(out.Results) != 1 || out.Results[0].Section != "" {
		t.Errorf("browse hit carries a section: %+v", out.Results)
	}
}

func TestVocabServesTheCorpusRegisters(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "context/categories.yaml", "categories:\n  bug: [defect]\n  refactor: []\n")
	writeFixture(t, root, "context/topics.yaml", "topics:\n  viewer: []\n  ui-ux: []\n")
	// an objective is a document: context/objectives/<id>.md
	for _, objective := range []string{"viewer", "cli"} {
		writeFixture(t, root, filepath.Join("context", "objectives", objective+".md"), "objective: "+objective+"\n")
	}
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/vocab", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out vocabResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Objectives) != 2 || out.Objectives[0] != "cli" || out.Objectives[1] != "viewer" {
		t.Errorf("objectives = %v", out.Objectives)
	}
	if len(out.Categories) != 2 || out.Categories[0] != "bug" || out.Categories[1] != "refactor" {
		t.Errorf("categories = %v", out.Categories)
	}
	if len(out.Topics) != 2 || out.Topics[0] != "ui-ux" || out.Topics[1] != "viewer" {
		t.Errorf("topics = %v", out.Topics)
	}
}

func TestVocabWithoutRegistersIsEmpty(t *testing.T) {
	h, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/vocab", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out vocabResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Objectives == nil || out.Categories == nil || out.Topics == nil {
		t.Errorf("want empty lists, got %+v", out)
	}
	if len(out.Objectives)+len(out.Categories)+len(out.Topics) != 0 {
		t.Errorf("want an empty vocabulary, got %+v", out)
	}
}

func TestVocabSurvivesAnUnreadableRegister(t *testing.T) {
	// a malformed register degrades to an empty list for that vocabulary, and the
	// other two still answer (the endpoint never fails the whole palette)
	root := t.TempDir()
	writeFixture(t, root, "context/topics.yaml", "topics: [oops\n")
	writeFixture(t, root, "context/categories.yaml", "categories:\n  bug: []\n")
	writeFixture(t, root, "context/objectives/viewer.md", "objective: viewer\n")
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/vocab", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out vocabResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Categories) != 1 || out.Categories[0] != "bug" {
		t.Errorf("categories = %v", out.Categories)
	}
	if len(out.Objectives) != 1 || out.Objectives[0] != "viewer" {
		t.Errorf("objectives = %v", out.Objectives)
	}
	if len(out.Topics) != 0 {
		t.Errorf("topics = %v, want empty for a malformed register", out.Topics)
	}
}

func TestVocabQueryEscapeIsNotNeededForFixedPath(t *testing.T) {
	// the endpoint takes no parameters; a stray query string is ignored
	h, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/vocab?root="+url.QueryEscape("/etc"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}
