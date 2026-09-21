package semantic

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
)

func encodedSnapshot() *Snapshot {
	return &Snapshot{
		Version:         1,
		Model:           ModelBase2M,
		RecipeVersion:   1,
		ManifestVersion: 1,
		DocCount:        2,
		DocHashes:       map[string]string{"a.md": "h1", "b.md": "h2"},
		SectionVectors: map[string][]float32{
			"a.md#x": {0.1, 0.2, 0.3},
			"a.md#y": {0.4, 0.5, 0.6},
			"b.md#z": {0.7, 0.8, 0.9},
		},
	}
}

func TestSnapshotSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := encodedSnapshot()

	if err := SaveSnapshot(root, want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file must not remain, got %v", err)
	}

	got := LoadSnapshot(root)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
	if !got.Matches(want.Model, want.RecipeVersion, want.ManifestVersion) {
		t.Error("loaded snapshot must match its own header")
	}
}

func TestSaveSnapshotNilRejected(t *testing.T) {
	if err := SaveSnapshot(t.TempDir(), nil); err == nil {
		t.Error("expected an error for a nil snapshot")
	}
}

func TestSnapshotLoadAbsentIsEmpty(t *testing.T) {
	s := LoadSnapshot(t.TempDir())
	if !s.Empty() {
		t.Errorf("absent snapshot must be empty, got %+v", s)
	}
	if s.Matches(ModelBase2M, 1, 1) {
		t.Error("empty snapshot must never match")
	}
}

func TestSnapshotLoadCorruptIsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, mdindex.CacheDir), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SnapshotPath(root), []byte("not a gob"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s := LoadSnapshot(root); !s.Empty() {
		t.Errorf("corrupt snapshot must be empty, got %+v", s)
	}
}

func TestSnapshotLoadFutureVersionIsEmpty(t *testing.T) {
	root := t.TempDir()
	future := encodedSnapshot()
	future.Version = 999
	if err := SaveSnapshot(root, future); err != nil {
		t.Fatal(err)
	}
	if s := LoadSnapshot(root); !s.Empty() {
		t.Errorf("future-version snapshot must load empty, got %+v", s)
	}
}

func TestSnapshotMatches(t *testing.T) {
	cases := []struct {
		name     string
		mut      func(*Snapshot)
		model    Model
		recipe   int
		manifest int
		want     bool
	}{
		{"exact", nil, ModelBase2M, 1, 1, true},
		{"model", func(s *Snapshot) { s.Model = ModelBase8M }, ModelBase2M, 1, 1, false},
		{"recipe", func(s *Snapshot) { s.RecipeVersion = 2 }, ModelBase2M, 1, 1, false},
		{"manifest", func(s *Snapshot) { s.ManifestVersion = 2 }, ModelBase2M, 1, 1, false},
		{"version", func(s *Snapshot) { s.Version = 2 }, ModelBase2M, 1, 1, false},
	}
	for _, c := range cases {
		s := encodedSnapshot()
		if c.mut != nil {
			c.mut(s)
		}
		if got := s.Matches(c.model, c.recipe, c.manifest); got != c.want {
			t.Errorf("%s: Matches(%s,%d,%d)=%v want %v", c.name, c.model, c.recipe, c.manifest, got, c.want)
		}
	}
	var nilSnap *Snapshot
	if nilSnap.Matches(ModelBase2M, 1, 1) {
		t.Error("nil snapshot must never match")
	}
	if (&Snapshot{}).Matches(ModelBase2M, 1, 1) {
		t.Error("zero snapshot must never match")
	}
}

func TestSnapshotLockContention(t *testing.T) {
	root := t.TempDir()
	release, ok := mdindex.RefreshLock(root)
	if !ok {
		t.Fatal("lock must be acquirable")
	}
	defer release()

	if err := SaveSnapshot(root, encodedSnapshot()); err != ErrSnapshotBusy {
		t.Errorf("concurrent save must return ErrSnapshotBusy, got %v", err)
	}
	if _, err := os.Stat(SnapshotPath(root)); !os.IsNotExist(err) {
		t.Error("failed save must not create a snapshot file")
	}
}

// corpusSections builds a small corpus: two docs, three sections total.
func corpusSections() ([]Section, map[string]string) {
	sections := []Section{
		{ID: "a.md#x", Path: "a.md", Text: "reciprocal rank fusion"},
		{ID: "a.md#y", Path: "a.md", Text: "controlled vocabulary"},
		{ID: "b.md#z", Path: "b.md", Text: "static embeddings cache"},
	}
	hashes := map[string]string{"a.md": "hash-a", "b.md": "hash-b"}
	return sections, hashes
}

func TestAddIncrementalReuseAndInvalidate(t *testing.T) {
	ctx := context.Background()
	first, err := New(ctx, testModel())
	if err != nil {
		t.Skipf("model unavailable (offline?): %v", err)
	}
	second, err := New(ctx, testModel())
	if err != nil {
		t.Skipf("model unavailable (offline?): %v", err)
	}

	sections, hashes := corpusSections()

	// Cold build: everything is embedded.
	out, reused, embedded := snap(t, first, sections, hashes, 1, &Snapshot{})
	if embedded != len(sections) || reused != 0 {
		t.Errorf("cold build: embedded=%d want %d, reused=%d want 0", embedded, len(sections), reused)
	}
	if !out.Matches(testModel(), RecipeVersion, 1) {
		t.Error("build output snapshot must match its header")
	}

	// Round-trip through disk and reuse on an identical corpus.
	root := t.TempDir()
	if err := SaveSnapshot(root, out); err != nil {
		t.Fatal(err)
	}
	base := LoadSnapshot(root)

	out2, reused, embedded := snap(t, second, sections, hashes, 1, base)
	if reused != len(sections) || embedded != 0 {
		t.Errorf("reuse: reused=%d want %d, embedded=%d want 0", reused, len(sections), embedded)
	}
	if count := second.Count(); count != len(sections) {
		t.Errorf("reused index must expose %d sections, got %d", len(sections), count)
	}

	// One document changes: only its sections are re-embedded.
	hashes2 := map[string]string{"a.md": "hash-a-2", "b.md": "hash-b"}
	out3, reused, embedded := snap(t, first, sections, hashes2, 1, out2)
	if reused != 1 || embedded != 2 {
		t.Errorf("changed doc: reused=%d want 1, embedded=%d want 2", reused, embedded)
	}
	_ = out3

	// Missing docHashes forces a full re-embed.
	_, reused, embedded = snap(t, first, sections, nil, 1, out)
	if reused != 0 || embedded != len(sections) {
		t.Errorf("docHashes nil: reused=%d want 0, embedded=%d want %d", reused, embedded, len(sections))
	}

	// Querying the reused collection keeps working end-to-end.
	hits, err := second.Query(ctx, "encodings and vectors", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != "b.md#z" {
		t.Errorf("reused index query mismatch, got %+v", hits)
	}
}

// snap is a tiny helper capturing the AddIncremental triple.
func snap(t *testing.T, ix *Index, sections []Section, hashes map[string]string, manifest int, base *Snapshot) (*Snapshot, int, int) {
	t.Helper()
	out, reused, embedded, err := ix.AddIncremental(context.Background(), sections, hashes, manifest, base)
	if err != nil {
		t.Fatal(err)
	}
	return out, reused, embedded
}
