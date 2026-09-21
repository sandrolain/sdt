package semantic

import (
	"context"
	"os"
	"testing"
)

// testModel picks a model for the test: SDT_EVAL_MODEL when set, otherwise the
// small default. The test skips when the model cannot be loaded (offline with
// no cached model), because the semantic branch is optional by design.
func testModel() Model {
	if m := os.Getenv("SDT_EVAL_MODEL"); m != "" {
		return Model(m)
	}
	return ModelBase2M
}

func TestUnknownModelRejected(t *testing.T) {
	if _, err := New(context.Background(), Model("NOPE")); err == nil {
		t.Error("expected an error for an unknown model id")
	}
}

func TestSectionRoundTripAndQuery(t *testing.T) {
	ctx := context.Background()
	ix, err := New(ctx, testModel())
	if err != nil {
		t.Skipf("model unavailable (offline?): %v", err)
	}
	if !ix.Available() {
		t.Fatal("index must be available after New")
	}
	if ix.Model() != testModel() {
		t.Errorf("model mismatch: %s", ix.Model())
	}

	sections := []Section{
		{ID: "context/a.md#x", Path: "context/a.md", Anchor: "x", Text: "reciprocal rank fusion of lexical and semantic rankings", Meta: map[string]string{"path": "context/a.md"}},
		{ID: "context/b.md#y", Path: "context/b.md", Anchor: "y", Text: "a controlled vocabulary of canonical subjects", Meta: map[string]string{"path": "context/b.md"}},
		{ID: "context/c.md#z", Text: ""}, // empty text is skipped
	}
	if err := ix.Add(ctx, sections); err != nil {
		t.Fatal(err)
	}
	if got := ix.Count(); got != 2 {
		t.Errorf("expected 2 stored sections, got %d", got)
	}

	hits, err := ix.Query(ctx, "rank fusion of two search rankings", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != "context/a.md#x" {
		t.Errorf("expected a.md#x first, got %+v", hits)
	}

	// k above the collection size must not error.
	if _, err := ix.Query(ctx, "anything", 100); err != nil {
		t.Errorf("k > count must clamp, got %v", err)
	}
}

func TestQueryUnavailableDegrades(t *testing.T) {
	var ix *Index
	if ix.Available() {
		t.Error("nil index must not be available")
	}
	if _, err := ix.Query(context.Background(), "x", 3); err == nil {
		t.Error("expected ErrUnavailable from a nil index")
	}
	if ix.Count() != 0 || ix.Model() != "" {
		t.Error("nil index must report empty state")
	}
}
