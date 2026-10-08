package semantic

import "context"

// BuildResult reports what a BuildSnapshot produced.
type BuildResult struct {
	// Path is the snapshot file that was written.
	Path string
	// Reused is the number of sections served from the previous snapshot.
	Reused int
	// Embedded is the number of sections freshly encoded.
	Embedded int
	// Snapshot is the persisted snapshot value.
	Snapshot *Snapshot
}

// BuildSnapshot loads the embedding model, (re)builds the vector snapshot over
// the given sections (reusing unchanged vectors from the persisted snapshot)
// and writes it back. It is the single build path shared by the CLI commands and
// the viewer so an embed/save is never duplicated.
//
// The model is loaded on every call; callers that only need the file should
// check persistence themselves. A snapshot write failure is best-effort (the
// search and the map stay correct and degrade on the next run) and never fails
// the build; only a model-load or encode error is returned.
func BuildSnapshot(ctx context.Context, root string, model Model, sections []Section, docHashes map[string]string, manifestVersion int) (*Index, BuildResult, error) {
	ix, err := New(ctx, model)
	if err != nil {
		return nil, BuildResult{}, err
	}
	base := LoadSnapshot(root)
	if !base.Matches(model, RecipeVersion, manifestVersion) {
		base = &Snapshot{} // header mismatch: full re-embed
	}
	out, reused, embedded, err := ix.AddIncremental(ctx, sections, docHashes, manifestVersion, base)
	if err != nil {
		return nil, BuildResult{}, err
	}
	// Best-effort persistence: a write failure only costs the next run.
	_ = SaveSnapshot(root, out) //nolint:errcheck // derived cache, degrade on failure
	return ix, BuildResult{
		Path:     SnapshotPath(root),
		Reused:   reused,
		Embedded: embedded,
		Snapshot: out,
	}, nil
}
