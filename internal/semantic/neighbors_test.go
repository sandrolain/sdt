package semantic

import "testing"

func TestCosine(t *testing.T) {
	if got := Cosine([]float32{1, 0}, []float32{1, 0}); got != 1 {
		t.Errorf("identical cosine = %v, want 1", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Errorf("orthogonal cosine = %v, want 0", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{1}); got != 0 {
		t.Errorf("mismatched-length cosine = %v, want 0", got)
	}
	if got := Cosine(nil, nil); got != 0 {
		t.Errorf("empty cosine = %v, want 0", got)
	}
}

func TestDocumentVectorsAveragesSections(t *testing.T) {
	s := &Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#one": {2, 0},
		"context/notes/a.md#two": {0, 2},
		"context/notes/b.md#one": {1, 1},
	}}
	docs := s.DocumentVectors()
	if got := docs["context/notes/a.md"]; len(got) != 2 || got[0] != 1 || got[1] != 1 {
		t.Errorf("a vector = %v, want [1 1]", got)
	}
	if got := docs["context/notes/b.md"]; got[0] != 1 || got[1] != 1 {
		t.Errorf("b vector = %v, want [1 1]", got)
	}
}

func TestNeighborsOrderingAndExclusion(t *testing.T) {
	s := &Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#body": {1, 0, 0},
		"context/notes/b.md#body": {0.9, 0.1, 0},
		"context/notes/c.md#body": {0, 1, 0},
	}}
	n := s.Neighbors("context/notes/a.md", 5)
	if len(n) != 2 {
		t.Fatalf("neighbors = %#v, want 2 (self excluded)", n)
	}
	if n[0].Path != "context/notes/b.md" || n[1].Path != "context/notes/c.md" {
		t.Fatalf("neighbors = %#v, want b then c", n)
	}
	if n[0].Score <= n[1].Score {
		t.Errorf("scores not descending: %#v", n)
	}
	// k limits the result.
	if got := s.Neighbors("context/notes/a.md", 1); len(got) != 1 || got[0].Path != "context/notes/b.md" {
		t.Errorf("limited neighbors = %#v, want only b", got)
	}
}

func TestNeighborsEmptySnapshot(t *testing.T) {
	var s Snapshot
	if n := s.Neighbors("context/notes/a.md", 3); n != nil {
		t.Errorf("neighbors on empty snapshot = %#v, want nil", n)
	}
	// A path with no vector yields nil even when other vectors exist.
	s2 := &Snapshot{SectionVectors: map[string][]float32{"context/notes/b.md#body": {1}}}
	if n := s2.Neighbors("context/notes/a.md", 3); n != nil {
		t.Errorf("neighbors for an absent path = %#v, want nil", n)
	}
}

func TestSimilarPairs(t *testing.T) {
	s := &Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#body": {1, 0},
		"context/notes/b.md#body": {0.99, 0.01},
		"context/notes/c.md#body": {0, 1},
	}}
	pairs := s.SimilarPairs([]string{"context/notes/a.md", "context/notes/b.md", "context/notes/c.md"}, 0.9)
	if len(pairs) != 1 || pairs[0].A != "context/notes/a.md" || pairs[0].B != "context/notes/b.md" {
		t.Fatalf("pairs = %#v, want only a/b", pairs)
	}
	// An absent path is skipped.
	if got := s.SimilarPairs([]string{"context/notes/missing.md", "context/notes/a.md"}, 0.0); len(got) != 0 {
		t.Errorf("pairs with an absent path = %#v, want none", got)
	}
}
