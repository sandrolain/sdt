package semantic

import (
	"reflect"
	"testing"
)

// TestDocumentVectorsDeterministic guards the U5 finding: the mean of a
// document's section vectors must not depend on Go map iteration order
// (float32 addition is non-associative). Values chosen so summation order
// changes the float32 result.
func TestDocumentVectorsDeterministic(t *testing.T) {
	vecs := map[string][]float32{
		"a.md#one":   {1e8, 1, 1},
		"a.md#two":   {1, 1, 1},
		"a.md#three": {1, 1, 1},
	}
	first := (&Snapshot{SectionVectors: vecs}).DocumentVectors()["a.md"]
	if first == nil {
		t.Fatal("no document vector for a.md")
	}
	for i := 0; i < 200; i++ {
		got := (&Snapshot{SectionVectors: vecs}).DocumentVectors()["a.md"]
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("DocumentVectors unstable at iteration %d: %v != %v", i, got, first)
		}
	}
}
