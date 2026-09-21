package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// benchCorpus writes n markdown docs under root/context and returns the scanned
// entries. The fixture writes happen before the measured phase via StopTimer.
func benchCorpus(b *testing.B, n int) (string, []*mdindex.Entry) {
	b.Helper()
	root := b.TempDir()
	dir := filepath.Join(root, "context", "analysis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	words := strings.Fields("alpha beta gamma delta epsilon zeta eta theta")
	for i := 0; i < n; i++ {
		word := words[i%len(words)]
		body := fmt.Sprintf("Section %d covering the %s concept with a distinctive marker-term-%02d planned for search verification.", i, word, i)
		raw := fmt.Sprintf("---\nkind: analysis\ntitle: Bench doc %02d\nsummary: Bench %02d about %s\ncreated: 2026-09-20\n---\n\n%s\n", i, i, word, body)
		p := filepath.Join(dir, fmt.Sprintf("bench-%02d.md", i))
		if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		b.Fatal(err)
	}
	return root, res.Manifest.EntriesSorted()
}

func benchEntriesIDs(entries []*mdindex.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

// BenchmarkStoreRebuildFresh measures a full on-disk build from entries.
func BenchmarkStoreRebuildFresh(b *testing.B) {
	root, entries := benchCorpus(b, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := RebuildStore(root, entries, benchEntriesIDs(entries), nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreRebuildIncremental measures the delta path: one doc changed
// and one removed against an existing base store.
func BenchmarkStoreRebuildIncremental(b *testing.B) {
	root, entries := benchCorpus(b, 200)
	if err := RebuildStore(root, entries, benchEntriesIDs(entries), nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		last := entries[len(entries)-1].ID
		if err := RebuildStore(root, entries, []string{entries[0].ID}, []string{last}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreOpen measures opening the store read-only for serving.
func BenchmarkStoreOpen(b *testing.B) {
	root, entries := benchCorpus(b, 200)
	if err := RebuildStore(root, entries, benchEntriesIDs(entries), nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix, err := OpenStore(root, entries)
		if err != nil {
			b.Fatal(err)
		}
		if err := ix.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemBuild is the baseline in-memory build for comparison.
func BenchmarkMemBuild(b *testing.B) {
	_, entries := benchCorpus(b, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix, err := NewFromEntries(entries)
		if err != nil {
			b.Fatal(err)
		}
		if err := ix.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
