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

// Add embeds and stores the given sections. Empty texts are skipped.
func (ix *Index) Add(ctx context.Context, sections []Section) error {
	if !ix.Available() {
		return ErrUnavailable
	}
	ids := make([]string, 0, len(sections))
	vecs := make([][]float32, 0, len(sections))
	metas := make([]map[string]string, 0, len(sections))
	docs := make([]string, 0, len(sections))
	for _, s := range sections {
		if s.Text == "" {
			continue
		}
		vec, err := ix.enc.Encode(s.Text)
		if err != nil {
			return fmt.Errorf("semantic: encode %s: %w", s.ID, err)
		}
		ids = append(ids, s.ID)
		vecs = append(vecs, vec)
		metas = append(metas, s.Meta)
		docs = append(docs, s.Text)
	}
	if len(ids) == 0 {
		return nil
	}
	return ix.col.Add(ctx, ids, vecs, metas, docs)
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
