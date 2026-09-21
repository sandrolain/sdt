package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/search"
	"github.com/sandrolain/sdt/internal/semantic"
)

// semanticOptions carries the opt-in semantic branch configuration. Enabled is
// false by default: /api/search always stays lexical unless it is switched on
// and the running model can be loaded.
type semanticOptions struct {
	// Enabled switches the branch on (flag --semantic or config search.semantic).
	Enabled bool
	// Model is the embedding model id; the empty value means the default
	// (BASE8M). Fine-tune via --semantic-model or config search.model.
	Model semantic.Model
}

// semanticOverrides is the precedence order used by newServerWith: explicit
// flag values win over the config file.
type semanticOverrides struct {
	enabled *bool
	model   *semantic.Model
}

// apply folds the overrides into the config-derived options.
func (o *semanticOverrides) apply(opts *semanticOptions) {
	if o.enabled != nil {
		opts.Enabled = *o.enabled
	}
	if o.model != nil {
		opts.Model = *o.model
	}
}

// viewerConfig is the subset of .sdt.yaml the viewer consumes; unknown keys are
// ignored.
type viewerConfig struct {
	Project string             `yaml:"project"`
	Group   string             `yaml:"group"`
	Search  viewerSearchConfig `yaml:"search"`
}

type viewerSearchConfig struct {
	// Semantic enables the optional /api/search?semantic=1 branch.
	Semantic bool `yaml:"semantic"`
	// Model overrides the embedding model id (default BASE8M). Empty disables
	// this override.
	Model string `yaml:"model"`
}

// loadViewerConfig reads root/.sdt.yaml for viewer-relevant keys. A missing or
// malformed file yields the zero value (semantic off) without error.
func loadViewerConfig(root string) viewerConfig {
	data, err := os.ReadFile(filepath.Join(root, sdtConfigFile)) //#nosec G304 -- user project config
	if err != nil {
		return viewerConfig{}
	}
	var cfg viewerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return viewerConfig{}
	}
	return cfg
}

// semanticOptionsFromConfig derives the branch options from the project config.
func semanticOptionsFromConfig(root string) semanticOptions {
	cfg := loadViewerConfig(root)
	opts := semanticOptions{Enabled: cfg.Search.Semantic}
	if cfg.Search.Model != "" {
		opts.Model = semantic.Model(cfg.Search.Model)
	}
	return opts
}

// setSemanticIndex swaps the semantic index under lock.
func (s *server) setSemanticIndex(sem *semantic.Index) {
	s.semMu.Lock()
	s.sem = sem
	s.semMu.Unlock()
}

// semanticIndex returns the current semantic index (nil when disabled or
// unavailable), mirroring the srch accessor.
func (s *server) semanticIndex() *semantic.Index {
	s.semMu.RLock()
	defer s.semMu.RUnlock()
	return s.sem
}

// rebuildSemantic (re)builds the semantic index over the given search index and
// manifest refresh, reusing the persisted vector snapshot so an unchanged
// corpus skips re-embedding. Any failure downgrades the branch to lexical
// (semantic nil) — the search stays correct either way. It is a no-op when the
// branch is disabled.
func (s *server) rebuildSemantic(ix *search.Index, refresh *mdindex.Refresh) {
	if !s.semOpts.Enabled {
		s.setSemanticIndex(nil)
		return
	}
	model := s.semOpts.Model
	if model == "" {
		model = semantic.ModelBase8M
	}
	sem, err := semantic.New(context.Background(), model)
	if err != nil {
		slog.Warn("sdtviewer: semantic index unavailable, serving lexical-only", "err", err, "model", model)
		s.setSemanticIndex(nil)
		return
	}
	base := semantic.LoadSnapshot(s.root)
	manifestV := mdindex.ManifestVersion
	if !base.Matches(model, semantic.RecipeVersion, manifestV) {
		base = &semantic.Snapshot{}
	}
	hashes := make(map[string]string)
	for _, e := range refresh.Manifest.EntriesSorted() {
		hashes[e.ID] = e.Hash
	}
	out, _, _, err := sem.AddIncremental(context.Background(), ix.SemanticSections(), hashes, manifestV, base)
	if err != nil {
		slog.Warn("sdtviewer: semantic fill failed, serving lexical-only", "err", err)
		s.setSemanticIndex(nil)
		return
	}
	// Best-effort persistence: a snapshot write failure only costs the next start.
	_ = semantic.SaveSnapshot(s.root, out) //nolint:errcheck // cache write, degrade on failure
	s.setSemanticIndex(sem)
}
