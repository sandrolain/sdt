package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// scanEntries scans the corpus at root and returns the sorted entry set.
func scanEntries(t *testing.T, root string) (*mdindex.ScanResult, []*mdindex.Entry) {
	t.Helper()
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return res, res.Manifest.EntriesSorted()
}

// reopen builds the serving index from the store (falling back to MemOnly when
// no store exists), mirroring what a caller does after RebuildStore.
func reopen(t *testing.T, root string) *Index {
	t.Helper()
	res, err := mdindex.Scan(root, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ix, err := OpenStore(root, res.Manifest.EntriesSorted())
	if err == ErrNoStore {
		ix, err = NewFromEntries(res.Manifest.EntriesSorted())
	}
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return ix
}

func TestStoreRebuildAndOpen(t *testing.T) {
	root := corpus(t)
	_, entries := scanEntries(t, root)
	// No store yet.
	if _, err := OpenStore(root, entries); err != ErrNoStore {
		t.Fatalf("OpenStore without store: err=%v want ErrNoStore", err)
	}
	// Full rebuild, then open.
	if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, err := os.Stat(StorePath(root)); err != nil {
		t.Fatalf("store dir missing after rebuild: %v", err)
	}
	ix := reopen(t, root)
	defer ix.Close()
	if len(ix.registry) != 7 {
		t.Errorf("stored registry has %d docs, want 7", len(ix.registry))
	}
	res, err := ix.Search("x9y8z7", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Results[0].Path != "context/wiki/alpha.md" {
		t.Errorf("store search: %+v", res)
	}
}

func TestStoreIncrementalDelta(t *testing.T) {
	root := corpus(t)
	scan1, entries := scanEntries(t, root)
	if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	// Change one doc (new unique term + summary) and remove another.
	alpha := filepath.Join(root, "context/wiki/alpha.md")
	content, err := os.ReadFile(alpha)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.ReplaceAll(string(content), "unique-term-x9y8z7", "unique-term-zz9")
	if err := os.WriteFile(alpha, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "context/wiki/beta.md")); err != nil {
		t.Fatal(err)
	}

	scan2, err := mdindex.Scan(root, scan1.Manifest)
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if len(scan2.Changed) != 1 || scan2.Changed[0] != "context/wiki/alpha.md" {
		t.Fatalf("changed = %v, want alpha only", scan2.Changed)
	}
	if len(scan2.Removed) != 1 || scan2.Removed[0] != "context/wiki/beta.md" {
		t.Fatalf("removed = %v, want beta only", scan2.Removed)
	}
	if err := RebuildStore(root, scan2.Manifest.EntriesSorted(), scan2.Changed, scan2.Removed); err != nil {
		t.Fatalf("delta rebuild: %v", err)
	}

	ix := reopen(t, root)
	defer ix.Close()
	if len(ix.registry) != 6 {
		t.Errorf("stored registry has %d docs, want 6", len(ix.registry))
	}
	// The new token is searchable, the superseded one is gone.
	if res, _ := ix.Search("zz9", "", "", "", "", "", "", 0); res.Total != 1 {
		t.Errorf("new token total = %d, want 1", res.Total)
	}
	if res, _ := ix.Search("x9y8z7", "", "", "", "", "", "", 0); res.Total != 0 {
		t.Errorf("old token total = %d, want 0 (doc changed)", res.Total)
	}
	// Removed doc is gone but the rest still ranks.
	if res, _ := ix.Search("tokens", "", "", "", "", "", "", 0); res.Total != 2 {
		t.Errorf("tokens total = %d, want 2 (alpha + notes)", res.Total)
	}
}

func TestStoreSchemaMismatchFallsBack(t *testing.T) {
	root := corpus(t)
	_, entries := scanEntries(t, root)
	if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// Forge an incompatible schema guard.
	path := filepath.Join(root, mdindex.CacheDir, "bleve.meta.json")
	garbage, _ := json.Marshal(map[string]any{"version": 1, "schema": "stale-schema"})
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(root, entries); err != ErrNoStore {
		t.Fatalf("OpenStore with stale schema: err=%v want ErrNoStore", err)
	}
	// The same stale meta forces RebuildStore to a fresh build, not a copy.
	if err := RebuildStore(root, entries, nil, nil); err != nil {
		t.Fatalf("rebuild after schema change: %v", err)
	}
	if ix := reopen(t, root); ix == nil || len(ix.registry) != 7 {
		t.Errorf("store unusable after rebuild: %v", ix)
	} else {
		ix.Close()
	}
}

func TestStoreCorruptDirFallsBack(t *testing.T) {
	root := corpus(t)
	_, entries := scanEntries(t, root)
	if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// Corrupt the on-disk store: the internal bolt file becomes garbage.
	boltPath := filepath.Join(StorePath(root), "index_meta.json")
	if err := os.WriteFile(boltPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(root, entries); err != ErrNoStore {
		t.Fatalf("OpenStore on corrupt store: err=%v want ErrNoStore", err)
	}
}

func TestStoreLockContention(t *testing.T) {
	root := corpus(t)
	_, entries := scanEntries(t, root)
	release, ok := mdindex.RefreshLock(root)
	if !ok {
		t.Fatal("could not take the refresh lock")
	}
	defer release()
	if err := RebuildStore(root, entries, nil, nil); err != ErrStoreBusy {
		t.Fatalf("RebuildStore under lock: err=%v want ErrStoreBusy", err)
	}
}

func TestStoreSwapKeepsOpenReaders(t *testing.T) {
	root := corpus(t)
	scan1, entries := scanEntries(t, root)
	if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	ix := reopen(t, root)
	res, err := ix.Search("x9y8z7", "", "", "", "", "", "", 0)
	if err != nil || res.Total != 1 {
		t.Fatalf("pre-swap search: %+v err=%v", res, err)
	}
	// A concurrent rebuild replaces the store under the open handle.
	scan2, err := mdindex.Scan(root, scan1.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := RebuildStore(root, scan2.Manifest.EntriesSorted(), scan2.Changed, scan2.Removed); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	// The already-open handle keeps serving the old inode.
	res, err = ix.Search("x9y8z7", "", "", "", "", "", "", 0)
	if err != nil || res.Total != 1 {
		t.Errorf("post-swap search on old handle: %+v err=%v", res, err)
	}
	ix.Close()
}

func TestLoadOrRebuild(t *testing.T) {
	charSince := func(ix *Index, term string) int64 {
		t.Helper()
		res, err := ix.Search(term, "", "", "", "", "", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		return res.Total
	}

	t.Run("no store no deltas builds and serves the store", func(t *testing.T) {
		root := corpus(t)
		_, entries := scanEntries(t, root)
		ix, err := LoadOrRebuild(root, entries, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ix.Close() }()
		if !storeDirExists(t, root) {
			t.Fatal("store not built on first use")
		}
		if n := charSince(ix, "unique-term-x9y8z7"); n != 1 {
			t.Errorf("stored search failed: %d", n)
		}
	})

	t.Run("deltas build and serve the store", func(t *testing.T) {
		root := corpus(t)
		_, entries := scanEntries(t, root)
		ix, err := LoadOrRebuild(root, entries, entriesIDs(entries), nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = ix.Close()
		if !storeDirExists(t, root) {
			t.Fatal("store not persisted")
		}
		// Second call with no deltas: serve the stored index.
		ix2, err := LoadOrRebuild(root, entries, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ix2.Close() }()
		if charSince(ix2, "unique-term-x9y8z7") != 1 {
			t.Errorf("stored index search failed")
		}
	})

	t.Run("corrupt store with no deltas gets rebuilt", func(t *testing.T) {
		root := corpus(t)
		_, entries := scanEntries(t, root)
		if _, err := LoadOrRebuild(root, entries, entriesIDs(entries), nil); err != nil {
			t.Fatal(err)
		}
		boltPath := filepath.Join(StorePath(root), "index_meta.json")
		if err := os.WriteFile(boltPath, []byte("garbage"), 0o644); err != nil {
			t.Fatal(err)
		}
		ix, err := LoadOrRebuild(root, entries, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ix.Close() }()
		if !storeDirExists(t, root) {
			t.Fatal("store not rebuilt after corruption")
		}
		if charSince(ix, "unique-term-x9y8z7") != 1 {
			t.Errorf("rebuilt store search failed")
		}
	})

	t.Run("failing rebuild serves in-memory", func(t *testing.T) {
		root := corpus(t)
		_, entries := scanEntries(t, root)
		// Block the cache dir so the rebuild cannot write; the read-only path
		// still serves an in-memory build.
		cacheDir := filepath.Join(root, mdindex.CacheDir)
		_ = os.Chmod(cacheDir, 0o500)
		defer func() { _ = os.Chmod(cacheDir, 0o700) }()
		ix, err := LoadOrRebuild(root, entries, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ix.Close() }()
		if n := charSince(ix, "unique-term-x9y8z7"); n != 1 {
			t.Errorf("in-memory fallback search failed: %d", n)
		}
	})
}

// storeDirExists reports whether a store directory is present on disk.
func storeDirExists(t *testing.T, root string) bool {
	t.Helper()
	st, err := os.Stat(StorePath(root))
	return err == nil && st.IsDir()
}

func TestStoreHelperErrors(t *testing.T) {
	t.Run("copyDir missing source", func(t *testing.T) {
		root := t.TempDir()
		if err := copyDir(filepath.Join(root, "nope"), filepath.Join(root, "dst")); err == nil {
			t.Fatal("copyDir on missing source must fail")
		}
	})
	t.Run("copyFile non-file source", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "srcdir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := copyFile(dir, filepath.Join(root, "out")); err == nil {
			t.Fatal("copyFile on a directory must fail")
		}
	})
	t.Run("rebuild missing root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "absent")
		if err := RebuildStore(root, nil, nil, nil); err == nil {
			t.Fatal("RebuildStore on missing root must fail")
		}
	})
	t.Run("unreadable doc is skipped", func(t *testing.T) {
		root := corpus(t)
		_, entries := scanEntries(t, root)
		// Corrupt one doc so its body load fails; the build must still succeed.
		bad := filepath.Join(root, "context/wiki/gamma.md")
		if err := os.WriteFile(bad, nil, 0o000); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(bad, 0o644) }()
		if err := RebuildStore(root, entries, entriesIDs(entries), nil); err != nil {
			t.Fatalf("rebuild with one unreadable doc: %v", err)
		}
	})
}

func entriesIDs(entries []*mdindex.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}
