package mdindex

import (
	"os"
	"path/filepath"
	"time"
)

// CacheDir is the derived cache directory inside the project root.
const CacheDir = ".sdt/cache"

// ManifestFile is the manifest path relative to the project root.
var ManifestFile = filepath.Join(CacheDir, "manifest.json")

// Refresh describes the outcome of EnsureFresh.
type Refresh struct {
	Manifest  *Manifest
	Changed   []string
	Removed   []string
	Unchanged int
	FromCache bool // true when a readable manifest was reused as the base
}

// EnsureFresh loads the cached manifest (if any), rescans the corpus and
// reports the documents that changed, persisting the refreshed manifest when
// something changed. It never mutates the markdown sources and never fails on a
// missing corpus.
//
// Concurrency contract: the CLI runs a single refresh per invocation (short
// processes); the long-lived viewer owns the write side and holds the refresh
// lock. A lock file guards concurrent writers; readers of an older manifest get
// a consistent snapshot because the write is atomic (temp file + rename).
func EnsureFresh(root string) (*Refresh, error) {
	base, err := Load(filepath.Join(root, ManifestFile))
	if err != nil {
		base = nil // a corrupt cache is rebuilt, never fatal
	}
	if base != nil {
		base.SetRoot(root)
	}
	res, err := Scan(root, base)
	if err != nil {
		return nil, err
	}
	refresh := &Refresh{
		Manifest:  res.Manifest,
		Changed:   res.Changed,
		Removed:   res.Removed,
		Unchanged: res.Unchanged,
		FromCache: base != nil,
	}
	if base == nil || len(res.Changed) > 0 || len(res.Removed) > 0 {
		if err := res.Manifest.Save(filepath.Join(root, ManifestFile)); err != nil {
			return refresh, err
		}
	}
	return refresh, nil
}

// RefreshLock is the single-writer lock used around a manifest write. The
// caller must call the returned release function. When the lock cannot be
// acquired the caller should proceed read-only (search still works on the last
// consistent manifest).
func RefreshLock(root string) (release func(), ok bool) {
	path := filepath.Join(root, CacheDir, "refresh.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301 -- derived cache dir
		return func() {}, false
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //#nosec G304 -- derived cache path
	if err != nil {
		return func() {}, false
	}
	if _, werr := f.WriteString(time.Now().UTC().Format(time.RFC3339) + "\n"); werr != nil {
		releaseLockFile(f, path)
		return func() {}, false
	}
	if cerr := f.Close(); cerr != nil {
		releaseLockFile(nil, path)
		return func() {}, false
	}
	return func() { releaseLockFile(nil, path) }, true
}

// releaseLockFile closes f (when non-nil) and removes the lock file, ignoring
// cleanup errors: a stale lock is harmless (callers degrade to read-only).
func releaseLockFile(f *os.File, path string) {
	if f != nil {
		//nolint:errcheck // best-effort lock cleanup; a stale lock degrades to read-only
		f.Close()
	}
	//nolint:errcheck // best-effort lock cleanup; a stale lock degrades to read-only
	os.Remove(path)
}
