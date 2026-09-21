// Package semantic provides the optional semantic branch of the sdt search:
// static embeddings (pure-Go, via go-potion) over section text, stored in
// chromem-go with exact cosine search. It is additive and degradable: when no
// model is available the caller keeps the lexical-only path.
//
// The collection is keyed by (model, recipe): a change in either invalidates
// the stored vectors, which are a derived cache (the source of truth stays the
// markdown files).
package semantic

import (
	"context"
	"errors"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
	potion "github.com/trengrj/go-potion"
)

// RecipeVersion is the embedding recipe version. Bump it whenever the text
// composition below changes, so cached vectors are rebuilt.
const RecipeVersion = 1

// Model selects the static embedding model. Multilingual is opt-in (large);
// the default is the small English model to keep cold start cheap.
type Model string

const (
	// ModelBase2M is the small English model (64 dims, ~8 MB).
	ModelBase2M Model = "BASE2M"
	// ModelBase8M is the balanced English model (256 dims, ~31 MB).
	ModelBase8M Model = "BASE8M"
	// ModelMultilingual128M is the 101-language model (256 dims, ~506 MB).
	ModelMultilingual128M Model = "MULTILINGUAL128M"
)

// potionModel maps the local model id to the go-potion constant.
func potionModel(m Model) (potion.Model, bool) {
	switch m {
	case ModelBase2M:
		return potion.BASE2M, true
	case ModelBase8M:
		return potion.BASE8M, true
	case ModelMultilingual128M:
		return potion.MULTILINGUAL128M, true
	default:
		return "", false
	}
}

// Section is one embeddable unit with its stable id and facet metadata.
type Section struct {
	ID     string // path#anchor
	Path   string
	Anchor string
	Text   string // the embedding recipe text
	Meta   map[string]string
}

// Index is the semantic index over sections. chromem-go collections are safe
// for concurrent reads; Add is called during construction before queries.
type Index struct {
	model Model
	enc   *potion.Potion
	col   *chromem.Collection
}

// ErrUnavailable is returned when the embedding model cannot be loaded (e.g.
// offline with no cached model). Callers degrade to lexical-only.
var ErrUnavailable = errors.New("semantic: embeddings unavailable")

// New loads the embedding model and returns an empty index. It may need network
// access on first use (the model is downloaded and cached by go-potion).
func New(ctx context.Context, model Model) (*Index, error) {
	pm, ok := potionModel(model)
	if !ok {
		return nil, fmt.Errorf("semantic: unknown model %q", model)
	}
	enc, err := potion.New(ctx, pm)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	db := chromem.NewDB()
	col, err := db.CreateCollection(collectionName(model), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic: create collection: %w", err)
	}
	return &Index{model: model, enc: enc, col: col}, nil
}

// collectionName keys a collection by model + recipe version.
func collectionName(m Model) string {
	return fmt.Sprintf("sections-%s-r%d", m, RecipeVersion)
}

// Available reports whether the index has a loaded encoder.
func (ix *Index) Available() bool { return ix != nil && ix.enc != nil }

// Model returns the active model id.
func (ix *Index) Model() Model {
	if ix == nil {
		return ""
	}
	return ix.model
}

// Add embeds and stores the given sections, reusing nothing (full embed).
// Empty texts are skipped. See AddIncremental for snapshot-driven reuse.
func (ix *Index) Add(ctx context.Context, sections []Section) error {
	_, _, _, err := ix.AddIncremental(ctx, sections, nil, 0, nil)
	return err
}

// AddIncremental embeds and stores the given sections, reusing stored vectors
// from base for sections whose document content hash is unchanged. out is the
// snapshot to persist afterwards. reused counts sections served from the
// snapshot; embedded counts freshly encoded sections. On error out is nil so
// the caller skips persistence.
//
// docHashes maps document id (Section.Path) -> its content hash; a section
// whose path is absent from docHashes is always re-embedded. base must be
// validated with Matches (or empty) before calling: any mismatch forces a full
// re-embed via the model/recipe guard below.
func (ix *Index) AddIncremental(ctx context.Context, sections []Section, docHashes map[string]string, manifestVersion int, base *Snapshot) (out *Snapshot, reused, embedded int, err error) {
	if !ix.Available() {
		return nil, 0, 0, ErrUnavailable
	}
	header := &Snapshot{
		Version:         snapshotVersion,
		Model:           ix.model,
		RecipeVersion:   RecipeVersion,
		ManifestVersion: manifestVersion,
		DocHashes:       make(map[string]string, len(docHashes)),
		SectionVectors:  make(map[string][]float32, len(sections)),
	}
	if len(docHashes) > 0 {
		header.DocCount = len(docHashes)
		for id, h := range docHashes {
			header.DocHashes[id] = h
		}
	}
	reuse := base.validFor(ix.model, RecipeVersion)

	ids := make([]string, 0, len(sections))
	vecs := make([][]float32, 0, len(sections))
	metas := make([]map[string]string, 0, len(sections))
	docs := make([]string, 0, len(sections))
	for _, s := range sections {
		if s.Text == "" {
			continue
		}
		var vec []float32
		if reuse {
			if v, ok := base.SectionVectors[s.ID]; ok && base.DocHashes[s.Path] == docHashes[s.Path] {
				vec = v
				reused++
			}
		}
		if vec == nil {
			v, err := ix.enc.Encode(s.Text)
			if err != nil {
				return nil, reused, embedded, fmt.Errorf("semantic: encode %s: %w", s.ID, err)
			}
			vec = v
			embedded++
		}
		ids = append(ids, s.ID)
		vecs = append(vecs, vec)
		metas = append(metas, s.Meta)
		docs = append(docs, s.Text)
		header.SectionVectors[s.ID] = vec
	}
	if len(ids) == 0 {
		return header, reused, embedded, nil
	}
	if err := ix.col.Add(ctx, ids, vecs, metas, docs); err != nil {
		return nil, reused, embedded, fmt.Errorf("semantic: add: %w", err)
	}
	return header, reused, embedded, nil
}

// validFor reports whether the snapshot vectors can be reused under the given
// model and recipe; version and manifest are validated by Matches.
func (s *Snapshot) validFor(model Model, recipeVersion int) bool {
	return s != nil &&
		s.Version == snapshotVersion &&
		s.Model == model &&
		s.RecipeVersion == recipeVersion
}

// Hit is one semantic result.
type Hit struct {
	ID         string
	Similarity float32
}

// Query embeds the query text and returns the top-k nearest sections.
func (ix *Index) Query(ctx context.Context, query string, k int) ([]Hit, error) {
	if !ix.Available() {
		return nil, ErrUnavailable
	}
	if k <= 0 {
		k = 20
	}
	if n := ix.col.Count(); k > n {
		k = n // chromem errors when k exceeds the collection size
	}
	if k == 0 {
		return nil, nil
	}
	vec, err := ix.enc.Encode(query)
	if err != nil {
		return nil, fmt.Errorf("semantic: encode query: %w", err)
	}
	res, err := ix.col.QueryEmbedding(ctx, vec, k, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic: query: %w", err)
	}
	hits := make([]Hit, 0, len(res))
	for _, r := range res {
		hits = append(hits, Hit{ID: r.ID, Similarity: r.Similarity})
	}
	return hits, nil
}

// Count returns the number of stored sections.
func (ix *Index) Count() int {
	if ix == nil || ix.col == nil {
		return 0
	}
	return ix.col.Count()
}
