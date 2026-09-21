package search

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/util"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// storeDir is the on-disk bleve directory name under .sdt/cache.
const storeDir = "bleve"

// storeMetaVersion invalidates stored meta when the meta schema changes.
const storeMetaVersion = 1

// storeMetaFile is the sidecar holding the store schema guard, next to the
// store directory under .sdt/cache.
var storeMetaFile = filepath.Join(mdindex.CacheDir, "bleve.meta.json")

// storeMeta is the version/schema guard written beside the store. A mismatch
// against the current mapping means the store must be rebuilt, never migrated.
type storeMeta struct {
	Version int    `json:"version"`
	Schema  string `json:"schema"`
}

// Sentinels the callers branch on. ErrNoStore means no usable store exists
// (callers fall back to an in-memory build); ErrStoreBusy means the single
// writer lock was not acquired (callers proceed read-only on the last store).
var (
	ErrNoStore   = errors.New("search: no persistent store")
	ErrStoreBusy = errors.New("search: store write locked")
)

// StorePath returns the on-disk bleve directory under <root>/.sdt/cache.
func StorePath(root string) string {
	return filepath.Join(root, mdindex.CacheDir, storeDir)
}

// storeSchemaFingerprint hashes the current index mapping (the exact schema the
// store is built with), so a mapping change invalidates the cached store.
func storeSchemaFingerprint() (string, error) {
	m, err := buildIndexMapping()
	if err != nil {
		return "", fmt.Errorf("search: mapping: %w", err)
	}
	b, err := util.MarshalJSON(m)
	if err != nil {
		return "", fmt.Errorf("search: fingerprint: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// loadStoreMeta reads the sidecar guard; a missing or invalid file returns
// (nil, nil) so callers treat the store as needing a rebuild.
func loadStoreMeta(root string) (*storeMeta, error) {
	data, err := os.ReadFile(filepath.Join(root, storeMetaFile)) //#nosec G304 -- derived cache path
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m storeMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil
	}
	if m.Version != storeMetaVersion {
		return nil, nil
	}
	return &m, nil
}

func saveStoreMeta(root, schema string) error {
	data, err := json.MarshalIndent(storeMeta{Version: storeMetaVersion, Schema: schema}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, storeMetaFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { //#nosec G306 -- derived cache file
		return err
	}
	return os.Rename(tmp, path)
}

// LoadOrRebuild returns a serving index backed by the persistent store, using
// the manifest scan (already refreshed by the caller) as the change signal.
//
//   - no deltas + fresh store ⇒ serve the store read-only (fast open);
//   - deltas reported ⇒ incrementally update the store first (best-effort);
//   - a busy/failing store write keeps the last store serving read-only;
//   - a missing or unusable store (absent/corrupt/schema change) is rebuilt
//     once; a second failure degrades to the in-memory build, never fatal.
//
// This is the single entry point CLI and viewer call to serve corpus search.
func LoadOrRebuild(root string, entries []*mdindex.Entry, changed, removed []string) (*Index, error) {
	stale := len(changed) > 0 || len(removed) > 0
	if stale {
		_ = RebuildStore(root, entries, changed, removed) //nolint:errcheck // busy ⇒ old store keeps serving
	}
	ix, err := OpenStore(root, entries)
	if err != ErrNoStore {
		return ix, err
	}
	// No usable store: rebuild once (full when absent/corrupt/schema-changed),
	// then serve it; a failing rebuild degrades to the in-memory build.
	if rerr := RebuildStore(root, entries, nil, nil); rerr == nil {
		if ix, oerr := OpenStore(root, entries); oerr == nil {
			return ix, nil
		}
	}
	return NewFromEntries(entries)
}

// OpenStore opens the persistent bleve store under <root>/.sdt/cache/bleve
// read-only and returns a serving index whose documents come from the store.
// entries supply the serving registry/sections (already on disk, never
// re-indexed here). The store is only reused when its schema guard matches the
// current mapping; otherwise it returns (nil, ErrNoStore) so the caller falls
// back to an in-memory build.
func OpenStore(root string, entries []*mdindex.Entry) (*Index, error) {
	meta, err := loadStoreMeta(root)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrNoStore
	}
	fp, err := storeSchemaFingerprint()
	if err != nil {
		return nil, err
	}
	if fp != meta.Schema {
		return nil, ErrNoStore
	}
	if _, err := os.Stat(StorePath(root)); err != nil {
		return nil, ErrNoStore
	}
	idx, err := bleve.OpenUsing(StorePath(root), map[string]interface{}{"read_only": true})
	if err != nil {
		return nil, ErrNoStore // corrupt or busy store: fall back / rebuild
	}
	ix, err := buildFromEntries(idx, entries, false)
	if err != nil {
		//nolint:errcheck // best-effort close after a half-built serving index
		idx.Close()
		return nil, err
	}
	return ix, nil
}

// RebuildStore writes (or updates) the persistent store under
// <root>/.sdt/cache/bleve, guarded by the single-writer lock and atomically
// swapped into place. When a valid base store exists the update is incremental
// (Index the changed docs, Delete the removed); otherwise the store is built
// fresh from entries. The in-memory path never changes: a caller that cannot
// write keeps searching on its current index.
func RebuildStore(root string, entries []*mdindex.Entry, changed, removed []string) error {
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("search: store root: %w", err)
	}
	release, ok := mdindex.RefreshLock(root)
	if !ok {
		return ErrStoreBusy
	}
	defer release()

	cacheDir := filepath.Join(root, mdindex.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil { //#nosec G301 -- derived cache dir
		return err
	}
	tmp, err := os.MkdirTemp(cacheDir, storeDir+".tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // clean up on every early return

	if err := rebuildInto(root, tmp, entries, changed, removed); err != nil {
		return err
	}

	fp, err := storeSchemaFingerprint()
	if err != nil {
		return err
	}
	if err := saveStoreMeta(root, fp); err != nil {
		return err
	}
	return swapStoreDir(tmp, StorePath(root))
}

// rebuildInto either copies the base store and applies the incremental delta,
// or builds a fresh store, writing into dir (the temp build directory).
func rebuildInto(root, dir string, entries []*mdindex.Entry, changed, removed []string) error {
	meta, err := loadStoreMeta(root)
	if err != nil {
		return err
	}
	base := StorePath(root)
	valid := meta != nil && meta.Version == storeMetaVersion
	if valid {
		if fp, ferr := storeSchemaFingerprint(); ferr != nil {
			return ferr
		} else if fp != meta.Schema {
			valid = false
		}
	}
	if st, serr := os.Stat(base); serr != nil || !st.IsDir() {
		valid = false
	}
	if valid {
		// Copy the base store to the temp dir, then apply doc-level deltas on
		// the copy (the live directory is never opened read-write). A failed
		// copy degrades to a fresh build.
		if cerr := copyDir(base, dir); cerr == nil {
			return applyDelta(dir, entries, changed, removed)
		}
		// A partial copy would poison a fresh build below: reset the dir.
		if rerr := os.RemoveAll(dir); rerr != nil {
			return rerr
		}
		if merr := os.MkdirAll(dir, 0o700); merr != nil {
			return merr
		}
	}
	// Fresh build.
	m, err := buildIndexMapping()
	if err != nil {
		return err
	}
	idx, err := bleve.New(dir, m)
	if err != nil {
		return fmt.Errorf("search: store new: %w", err)
	}
	defer func() {
		//nolint:errcheck // best-effort close; content is already persisted
		idx.Close()
	}()
	for _, e := range entries {
		if err := e.LoadBody(); err != nil {
			continue // unreadable doc is skipped, never fatal
		}
		if err := idx.Index(e.ID, docFromEntry(e)); err != nil {
			return fmt.Errorf("search: store index %s: %w", e.ID, err)
		}
	}
	return nil
}

// applyDelta opens dir read-write and applies the changed/removed doc deltas to
// it (the copy of a valid base store).
func applyDelta(dir string, entries []*mdindex.Entry, changed, removed []string) error {
	idx, err := bleve.OpenUsing(dir, nil)
	if err != nil {
		return fmt.Errorf("search: store open delta: %w", err)
	}
	defer func() {
		//nolint:errcheck // persistence happened when the index flushed on Close
		idx.Close()
	}()
	byID := make(map[string]*mdindex.Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	for _, id := range removed {
		if err := idx.Delete(id); err != nil {
			return fmt.Errorf("search: store delete %s: %w", id, err)
		}
	}
	for _, id := range changed {
		e, ok := byID[id]
		if !ok {
			continue // removed in the same pass: nothing to index
		}
		if err := e.LoadBody(); err != nil {
			continue // unreadable doc is skipped, never fatal
		}
		if err := idx.Index(id, docFromEntry(e)); err != nil {
			return fmt.Errorf("search: store index %s: %w", id, err)
		}
	}
	return nil
}

// swapStoreDir atomically replaces dst with src on the same filesystem: src is
// renamed into place and the previous dst moved aside then removed. A reader
// opening during the swap sees no store and falls back; an already-open handle
// keeps serving the old inode until closed.
func swapStoreDir(src, dst string) error {
	aside := dst + ".old"
	if err := os.Rename(dst, aside); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(aside, dst) //nolint:errcheck // best-effort restore
		return err
	}
	//nolint:errcheck // best-effort cleanup of the replaced store
	os.RemoveAll(aside)
	return nil
}

// copyDir copies the directory tree at src into a fresh dst.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies one regular file preserving its mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //#nosec G304 -- derived cache tree copy
	if err != nil {
		return err
	}
	defer func() {
		//nolint:errcheck // read-only source
		in.Close()
	}()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()) //#nosec G304 -- derived cache tree copy
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() //nolint:errcheck // propagate the write error instead
		return err
	}
	return out.Close()
}
