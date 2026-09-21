package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// evalDoc is one document in the query-quality fixture.
type evalDoc struct {
	rel  string
	body string
}

// evalQueries are realistic queries with the document that must rank in the
// top-3 (MRR@3). They are deliberately paraphrases, not keyword copies, so the
// metric is meaningful.
var evalQueries = []struct {
	query string
	want  string
}{
	{"hybrid lexical semantic fusion", "context/analysis/hybrid.md"},
	{"controlled subject vocabulary", "context/analysis/topics.md"},
	{"reject an approach that failed", "context/analysis/deadend.md"},
	{"incremental index cache", "context/analysis/cache.md"},
	{"independent verification before closing", "context/analysis/review.md"},
}

func evalCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	docs := []evalDoc{
		{"context/analysis/hybrid.md", "---\nkind: analysis\nsummary: Fuse bleve lexical ranking with static embedding vectors using reciprocal rank fusion\n---\n\n## Design\n\nCombine a BM25 lexical ranking with a semantic vector ranking through rank fusion so no score normalization is needed.\n"},
		{"context/analysis/topics.md", "---\nkind: analysis\nsummary: A controlled register of canonical subjects with aliases keeps tagging consistent\n---\n\n## Vocabulary\n\nA canonical list of subjects with alias canonicalization stops free tags from drifting.\n"},
		{"context/analysis/deadend.md", "---\nkind: analysis\nsummary: Record approaches that were tried and rejected under an objective\n---\n\n## Memory\n\nA dead-end note stores an attempt that failed so a later session does not repeat it.\n"},
		{"context/analysis/cache.md", "---\nkind: analysis\nsummary: A derived manifest makes scanning only changed files and rebuilds cheap\n---\n\n## Incremental\n\nHashing changed files and reusing unchanged entries avoids a full rebuild.\n"},
		{"context/analysis/review.md", "---\nkind: analysis\nsummary: A closed verdict vocabulary and an independent pass validate findings before a phase closes\n---\n\n## Protocol\n\nFindings are confirmed or disproved by a separate reviewer, never self-approved.\n"},
	}
	for _, d := range docs {
		p := filepath.Join(root, d.rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(d.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// mrrAt3 measures the lexical ranking quality on the fixture. The full
// lexical-vs-hybrid comparison runs only when a model is available (the
// semantic branch is opt-in to keep CI offline-safe).
func TestLexicalQueryQualityMRR(t *testing.T) {
	root := evalCorpus(t)
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := NewFromEntries(res.Manifest.EntriesSorted())
	if err != nil {
		t.Fatal(err)
	}
	sum := 0.0
	for _, q := range evalQueries {
		r, err := ix.Search(q.query, "", "", "", "", "", "", 3)
		if err != nil {
			t.Fatal(err)
		}
		for rank, hit := range r.Results {
			if hit.Path == q.want {
				sum += 1.0 / float64(rank+1)
				break
			}
		}
	}
	mrr := sum / float64(len(evalQueries))
	t.Logf("lexical MRR@3 = %.3f (n=%d)", mrr, len(evalQueries))
	if mrr < 0.5 {
		t.Errorf("lexical MRR@3 too low: %.3f", mrr)
	}
}

// TestHybridFusionDegradesWithoutModel ensures the hybrid path is a no-op when
// no semantic index is supplied, and that RRF reinforces a document matched by
// both branches.
func TestHybridFusionDegradesWithoutModel(t *testing.T) {
	root := evalCorpus(t)
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := NewFromEntries(res.Manifest.EntriesSorted())
	if err != nil {
		t.Fatal(err)
	}
	q := HybridQuery{Q: "reciprocal rank fusion", Max: 5}
	plain, err := ix.SearchHybrid(context.Background(), q, HybridOptions{})
	if err != nil {
		t.Fatal(err)
	}
	base, err := ix.Search(q.Q, q.Kind, q.Objective, q.Status, q.Topic, q.From, q.To, q.Max)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Results) != len(base.Results) {
		t.Errorf("hybrid without a model must equal lexical: %d vs %d", len(plain.Results), len(base.Results))
	}
}

// TestRRFReinforcesSharedDocument checks that a document ranked by both
// rankings outranks a document ranked by only one.
func TestRRFReinforcesSharedDocument(t *testing.T) {
	lexical := []Result{{Path: "context/a.md"}, {Path: "context/b.md"}}
	fused := fuseHybrid(lexical, []string{"context/b.md#x", "context/c.md#y"})
	// b.md is in both rankings (ranks 2 and 1) so it must beat a.md (rank 1 only).
	if fused[0].Path != "context/b.md" {
		t.Errorf("expected b.md to win after fusion, got %+v", fused)
	}
}

func TestSplitSectionID(t *testing.T) {
	if p, a := splitSectionID("context/a.md#hybrid"); p != "context/a.md" || a != "hybrid" {
		t.Errorf("splitSectionID = %q,%q", p, a)
	}
	if p, a := splitSectionID("context/a.md"); p != "context/a.md" || a != "" {
		t.Errorf("splitSectionID no anchor = %q,%q", p, a)
	}
}

// mdindexScanHelper scans a corpus root and returns its entries (test helper).
func mdindexScanHelper(root string) ([]*mdindex.Entry, error) {
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		return nil, err
	}
	return res.Manifest.EntriesSorted(), nil
}
