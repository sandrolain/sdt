package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/sandrolain/sdt/internal/search"
	"github.com/sandrolain/sdt/internal/semantic"
)

// probeViewerSemantic skips when the small BASE2M model cannot load so the
// viewer semantic tests stay offline-friendly.
func probeViewerSemantic(t *testing.T) {
	t.Helper()
	if _, err := semantic.New(context.Background(), semantic.ModelBase2M); err != nil {
		t.Skipf("BASE2M model unavailable (offline?): %v", err)
	}
}

// enableViewerSemantic writes a .sdt.yaml enabling the branch with BASE2M.
func enableViewerSemantic(t *testing.T, root string) {
	t.Helper()
	writeFixture(t, root, ".sdt.yaml", "project: viewerTest\nsearch:\n  semantic: true\n  model: BASE2M\n")
}

// searchGET runs a GET against h and decodes the search.Results payload.
func searchGET(t *testing.T, h http.Handler, query string) search.Results {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(query), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSearchSemanticDisabledFallsBackToLexical covers the default contract:
// no config (or config off) means ?semantic=1 still serves plain lexical
// results and never errors.
func TestSearchSemanticDisabledFallsBackToLexical(t *testing.T) {
	root := makeSearchCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	with := searchGET(t, h, "tokens&semantic=1")
	without := searchGET(t, h, "tokens")
	if with.Total != without.Total || with.Total != 4 {
		t.Errorf("disabled branch: semantic=1 total=%d, default total=%d, want 4", with.Total, without.Total)
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("disabled branch must not create a vector snapshot")
	}
}

// TestSearchSemanticHybridAndRebuild covers the enabled branch: hybrid results
// over the persisted snapshot, filter application, and snapshot refresh on
// corpus change.
func TestSearchSemanticHybridAndRebuild(t *testing.T) {
	probeViewerSemantic(t)
	root := makeSearchCorpus(t)
	enableViewerSemantic(t, root)

	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	if s.semanticIndex() == nil {
		t.Fatal("enabled server must build a semantic index")
	}
	if st, err := os.Stat(semantic.SnapshotPath(root)); err != nil || st.IsDir() {
		t.Fatalf("enabled server must persist the vector snapshot: %v", err)
	}
	firstHash := semantic.LoadSnapshot(root).DocHashes["context/wiki/model.md"]
	if firstHash == "" {
		t.Fatal("snapshot must track the model page hash")
	}

	h := s.mux()

	// Hybrid response shape: same Results contract as lexical.
	hybrid := searchGET(t, h, "tokens&semantic=1")
	if hybrid.Total < 1 || len(hybrid.Results) == 0 {
		t.Fatalf("hybrid search empty: %+v", hybrid)
	}
	for _, r := range hybrid.Results {
		if r.Path == "" {
			t.Errorf("hybrid result missing path: %+v", r)
		}
	}

	// Filters still apply on the hybrid branch.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search?q=tokens&semantic=1&kind=wiki", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var filtered search.Results
	if err := json.Unmarshal(rec.Body.Bytes(), &filtered); err != nil {
		t.Fatal(err)
	}
	for _, r := range filtered.Results {
		if r.Kind != "wiki" {
			t.Errorf("filtered hybrid hit outside kind=wiki: %+v", r)
		}
	}

	// Rebuild on a corpus change refreshes the snapshot in place.
	model := root + "/context/wiki/model.md"
	data, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, bytes.ReplaceAll(data, []byte("tokens"), []byte("goldenhybridterm")), 0o644); err != nil {
		t.Fatal(err)
	}
	s.rebuildSearch([]string{"context/wiki/model.md"})
	if s.semanticIndex() == nil {
		t.Fatal("semantic index must survive a corpus rebuild")
	}
	if h := semantic.LoadSnapshot(root).DocHashes["context/wiki/model.md"]; h == "" || h == firstHash {
		t.Errorf("rebuild must update the changed doc hash: old=%q new=%q", firstHash, h)
	}
	if out := searchGET(t, h, "goldenhybridterm&semantic=1"); out.Total == 0 {
		t.Errorf("rebuilt semantic branch must find the changed term, got %+v", out)
	}
}

// TestSearchSemanticUnavailableModelFallsBack covers an explicit invalid model:
// the branch stays lexical, the endpoint never errors.
func TestSearchSemanticUnavailableModelFallsBack(t *testing.T) {
	root := makeSearchCorpus(t)
	writeFixture(t, root, ".sdt.yaml", "search:\n  semantic: true\n  model: NOPE\n")
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	if s.semanticIndex() != nil {
		t.Fatal("unknown model must leave the semantic branch nil")
	}
	h := s.mux()
	out := searchGET(t, h, "tokens&semantic=1")
	if out.Total != 4 {
		t.Errorf("unavailable model must degrade to lexical, total=%d want 4", out.Total)
	}
	if _, err := os.Stat(semantic.SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("failed semantic build must not write a snapshot")
	}
}
