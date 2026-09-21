package semantic

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// snapshotVersion is the on-disk snapshot format version. Bump it whenever the
// snapshot schema changes so stale snapshots are rebuilt instead of decoded.
const snapshotVersion = 1

// snapshotFile is the snapshot file path relative to the project root.
var snapshotFile = filepath.Join(mdindex.CacheDir, "vectors.gob.gz")

// ErrSnapshotBusy is returned when the snapshot write lock cannot be acquired;
// callers keep the last snapshot (or skip persistence) instead of failing.
var ErrSnapshotBusy = errors.New("semantic: snapshot write locked")

// Snapshot is the durable, incremental vector cache: the embedding result per
// section plus, per document, the content hash that produced them. A change in
// model, recipe or manifest version (the header guard) invalidates everything;
// a per-document hash change invalidates that document only. It is a derived
// cache (the markdown files are the source of truth) and encodes as gob.
type Snapshot struct {
	// Version is the snapshot schema version (snapshotVersion).
	Version int
	// Model is the embedding model id that produced the vectors.
	Model Model
	// RecipeVersion is the embedding recipe version (RecipeVersion).
	RecipeVersion int
	// ManifestVersion is the mdindex.ManifestVersion of the scanned corpus.
	ManifestVersion int
	// DocCount is the number of documents captured (header sanity info).
	DocCount int
	// DocHashes maps document id -> its content hash at capture time.
	DocHashes map[string]string
	// SectionVectors maps section id -> its embedding vector.
	SectionVectors map[string][]float32
}

// SnapshotPath returns the snapshot path under <root>/.sdt/cache.
func SnapshotPath(root string) string {
	return filepath.Join(root, snapshotFile)
}

// Matches reports whether the snapshot can be reused for the given model,
// recipe and manifest versions. A zero-value snapshot never matches.
func (s *Snapshot) Matches(model Model, recipeVersion, manifestVersion int) bool {
	return s != nil &&
		s.Version == snapshotVersion &&
		s.Model == model &&
		s.RecipeVersion == recipeVersion &&
		s.ManifestVersion == manifestVersion
}

// Empty reports whether the snapshot holds no reusable vectors.
func (s *Snapshot) Empty() bool {
	return s == nil || len(s.SectionVectors) == 0
}

// LoadSnapshot reads the snapshot under root. An absent, corrupt or
// version-mismatched file yields an empty snapshot (never an error): the caller
// then falls back to a full embed, then persists a fresh snapshot.
func LoadSnapshot(root string) *Snapshot {
	data, err := os.ReadFile(SnapshotPath(root)) //#nosec G304 -- derived cache path
	if err != nil {
		return &Snapshot{}
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return &Snapshot{}
	}
	var s Snapshot
	if err := gob.NewDecoder(zr).Decode(&s); err != nil {
		return &Snapshot{}
	}
	if s.Version != snapshotVersion {
		return &Snapshot{}
	}
	return &s
}

// SaveSnapshot writes the snapshot under root atomically (temp file + rename in
// the cache dir) under the single-writer refresh lock. A busy lock returns
// ErrSnapshotBusy and leaves the previous snapshot untouched.
func SaveSnapshot(root string, s *Snapshot) error {
	if s == nil {
		return errors.New("semantic: nil snapshot")
	}
	release, ok := mdindex.RefreshLock(root)
	if !ok {
		return ErrSnapshotBusy
	}
	defer release()

	cacheDir := filepath.Join(root, mdindex.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil { //#nosec G301 -- derived cache dir
		return err
	}
	path := SnapshotPath(root)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //#nosec G304 -- derived cache file
	if err != nil {
		return err
	}
	closeAndRemove := func() {
		_ = f.Close()      //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmp) //nolint:errcheck // best-effort cleanup
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := gob.NewEncoder(gz).Encode(s); err != nil {
		_ = gz.Close() //nolint:errcheck // best-effort cleanup
		closeAndRemove()
		return fmt.Errorf("semantic: snapshot encode: %w", err)
	}
	if err := gz.Close(); err != nil {
		closeAndRemove()
		return fmt.Errorf("semantic: snapshot gzip: %w", err)
	}
	if _, err := io.Copy(f, &buf); err != nil {
		closeAndRemove()
		return fmt.Errorf("semantic: snapshot write: %w", err)
	}
	if err := f.Close(); err != nil {
		closeAndRemove()
		return fmt.Errorf("semantic: snapshot close: %w", err)
	}
	return os.Rename(tmp, path)
}
