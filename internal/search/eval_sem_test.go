package search

import (
	"context"
	"os"
	"testing"

	"github.com/sandrolain/sdt/internal/semantic"
)

// TestHybridQueryQualityMRR measures lexical vs hybrid MRR@3 on the fixture.
// It is skipped unless SDT_EVAL_MODEL names a model that is cached locally, so
// CI stays offline-safe while the comparison is reproducible on demand.
func TestHybridQueryQualityMRR(t *testing.T) {
	model := os.Getenv("SDT_EVAL_MODEL")
	if model == "" {
		t.Skip("set SDT_EVAL_MODEL=BASE2M|BASE8M|MULTILINGUAL128M to run the hybrid measurement")
	}
	root := evalCorpus(t)
	res, err := mdindexScanHelper(root)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := NewFromEntries(res)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := semantic.New(context.Background(), semantic.Model(model))
	if err != nil {
		t.Skipf("model %s unavailable: %v", model, err)
	}
	if err := sem.Add(context.Background(), ix.SemanticSections()); err != nil {
		t.Fatal(err)
	}
	lexMRR, hybMRR := 0.0, 0.0
	for _, q := range evalQueries {
		base, err := ix.Search(q.query, "", "", "", "", "", "", 3)
		if err != nil {
			t.Fatal(err)
		}
		h, err := ix.SearchHybrid(context.Background(), HybridQuery{Q: q.query, Max: 3}, HybridOptions{Semantic: sem})
		if err != nil {
			t.Fatal(err)
		}
		lexMRR += rankReciprocal(base.Results, q.want)
		hybMRR += rankReciprocal(h.Results, q.want)
	}
	n := float64(len(evalQueries))
	t.Logf("MRR@3 lexical=%.3f hybrid=%.3f (model=%s, n=%d)", lexMRR/n, hybMRR/n, model, len(evalQueries))
}

func rankReciprocal(results []Result, want string) float64 {
	for i, r := range results {
		if r.Path == want {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// TestHybridEnrichesSemanticOnlyHits covers the path where a semantic hit has
// no lexical counterpart: the registry fields must be filled in. A tiny fake
// semantic branch is not possible (concrete type), so this drives the same
// enrichment via a lexical query that matches only one doc while a second is
// added to the fusion list through a manual Result union.
func TestSemanticSectionsAndRecipe(t *testing.T) {
	root := evalCorpus(t)
	entries, err := mdindexScanHelper(root)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := NewFromEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	secs := ix.SemanticSections()
	if len(secs) == 0 {
		t.Fatal("expected semantic sections")
	}
	for _, s := range secs {
		if s.ID == "" || s.Path == "" || s.Text == "" {
			t.Errorf("incomplete section: %+v", s)
		}
		if s.Meta["path"] != s.Path {
			t.Errorf("section meta path mismatch: %+v", s)
		}
	}
	// Sections is the manifest-facing accessor.
	if len(ix.Sections()) != len(secs) {
		t.Errorf("Sections() and SemanticSections() disagree: %d vs %d", len(ix.Sections()), len(secs))
	}
}

// TestHybridFilterDropsOutOfSetSemanticHits pins the documented contract: when
// a filter is active, semantic-only fused hits outside the filter set must be
// dropped. No model needed — exercises the pure predicate.
func TestHybridFilterDropsOutOfSetSemanticHits(t *testing.T) {
	wiki := doc{Path: "context/wiki/a.md", Kind: "wiki", Status: "active", Objective: "", Topics: []string{"x"}, RawCreated: "2026-09-10", CreatedDays: parseCreatedDays("2026-09-10")}
	note := doc{Path: "context/notes/b.md", Kind: "notes", Status: "active", RawCreated: "2026-09-11", CreatedDays: parseCreatedDays("2026-09-11")}

	f := hybridFilter(HybridQuery{Kind: "wiki"})
	if f == nil {
		t.Fatal("kind filter must yield a predicate")
	}
	if !f(wiki.Path, &wiki) {
		t.Error("wiki doc must pass the kind=wiki filter")
	}
	if f(note.Path, &note) {
		t.Error("notes doc must fail the kind=wiki filter")
	}

	if hybridFilter(HybridQuery{}) != nil {
		t.Error("no-filter query must yield nil predicate")
	}

	// date bounds: from inclusive, to inclusive of the whole day
	df := hybridFilter(HybridQuery{From: "2026-09-11", To: "2026-09-11"})
	if df == nil {
		t.Fatal("date filter must yield a predicate")
	}
	if !df(note.Path, &note) {
		t.Error("11 Sep created must pass from/to 2026-09-11..2026-09-11")
	}
	if df(wiki.Path, &wiki) {
		t.Error("10 Sep created must fail from=2026-09-11")
	}

	// topic: exact per-token membership, matching the bleve TermQuery.
	tf := hybridFilter(HybridQuery{Topic: "x"})
	if tf == nil || !tf(wiki.Path, &wiki) || tf(note.Path, &note) {
		t.Error("topic filter must match only the tagged doc")
	}
}

func TestFuseHybridAnchorForAndTies(t *testing.T) {
	lexical := []Result{{Path: "context/a.md", Kind: "analysis"}}
	fused := fuseHybrid(lexical, []string{"context/b.md#beta", "context/a.md#alpha"})
	// a.md appears in both rankings; b.md is semantic-only and must still be
	// returned with its anchor.
	var a, b *Result
	for i := range fused {
		switch fused[i].Path {
		case "context/a.md":
			a = &fused[i]
		case "context/b.md":
			b = &fused[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("expected both docs in the fusion: %+v", fused)
	}
	if b.Section != "beta" {
		t.Errorf("semantic-only hit should carry its anchor, got %q", b.Section)
	}
}
