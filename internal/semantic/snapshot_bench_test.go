package semantic

import (
	"context"
	"fmt"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// benchDocCount and benchSectionCount mirror the real project corpus
// (491 documents, 3015 sections; 6 sections per document).
const (
	benchDocCount     = 491
	benchSectionCount = 491 * 6
	benchDim          = 256
)

// benchSections builds the benchmark corpus: stable ids and per-doc hashes so
// snapshot reuse matches on the second pass.
func benchSections() ([]Section, map[string]string) {
	sections := make([]Section, 0, benchSectionCount)
	hashes := make(map[string]string, benchDocCount)
	for i := 0; i < benchDocCount; i++ {
		id := fmt.Sprintf("context/doc-%04d.md", i)
		hashes[id] = fmt.Sprintf("hash-%d", i)
		for j := 0; j < benchSectionCount/benchDocCount; j++ {
			anchor := fmt.Sprintf("s%02d", j)
			sections = append(sections, Section{
				ID:   id + "#" + anchor,
				Path: id,
				Text: fmt.Sprintf("benchmark section %s of %s: static embeddings over a heading, a summary and its body text", anchor, id),
				Meta: map[string]string{"path": id, "section": anchor},
			})
		}
	}
	return sections, hashes
}

// seedVector builds a deterministic dim-sized vector in place of an embed.
func seedVector(seed, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(seed%97) / 97
	}
	return v
}

// benchSnapshot precomputes and persists a full snapshot (the state after a
// previous run) so the reuse pass has vectors ready.
func benchSnapshot(root string, sections []Section, hashes map[string]string) *Snapshot {
	s := &Snapshot{
		Version:         snapshotVersion,
		Model:           ModelBase2M,
		RecipeVersion:   RecipeVersion,
		ManifestVersion: mdindex.ManifestVersion,
		DocCount:        benchDocCount,
		DocHashes:       hashes,
		SectionVectors:  make(map[string][]float32, len(sections)),
	}
	for i, sec := range sections {
		s.SectionVectors[sec.ID] = seedVector(i, benchDim)
	}
	return s
}

// BenchmarkSnapshotFullEmbed measures one cold build of the whole corpus
// (model load + 2946 embeds + chromem insert). Record with -benchtime=1x.
func BenchmarkSnapshotFullEmbed(b *testing.B) {
	ix, err := New(context.Background(), ModelBase2M)
	if err != nil {
		b.Skipf("model unavailable (offline?): %v", err)
	}
	sections, hashes := benchSections()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := ix.AddIncremental(ctx, sections, hashes, mdindex.ManifestVersion, &Snapshot{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSnapshotReuse measures a warm start on an unchanged corpus: model
// load + snapshot load + full vector reuse (zero embeds) + chromem insert.
// Record with -benchtime=1x.
func BenchmarkSnapshotReuse(b *testing.B) {
	sections, hashes := benchSections()
	root := b.TempDir()
	if err := SaveSnapshot(root, benchSnapshot(root, sections, hashes)); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix, err := New(ctx, ModelBase2M)
		if err != nil {
			b.Skipf("model unavailable (offline?): %v", err)
		}
		out, reused, embedded, err := ix.AddIncremental(ctx, sections, hashes, mdindex.ManifestVersion, LoadSnapshot(root))
		if err != nil {
			b.Fatal(err)
		}
		if reused != benchSectionCount || embedded != 0 {
			b.Fatalf("reuse pass: reused=%d embedded=%d", reused, embedded)
		}
		_ = out
	}
}

// BenchmarkSnapshotLoad measures snapshot load+decode alone (gzip + gob).
func BenchmarkSnapshotLoad(b *testing.B) {
	sections, hashes := benchSections()
	root := b.TempDir()
	if err := SaveSnapshot(root, benchSnapshot(root, sections, hashes)); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if s := LoadSnapshot(root); s.Empty() {
			b.Fatal("snapshot must load with vectors")
		}
	}
}
