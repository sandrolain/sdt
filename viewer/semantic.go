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
	// Enabled switches the search branch on (flag --semantic or config
	// search.semantic).
	Enabled bool
	// BuildOnDemand lets the viewer build the vector snapshot when the semantic
	// map asks for it and none exists yet (config search.semantic_build, default
	// true). It is independent of Enabled: the map is its own consumer.
	BuildOnDemand bool
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
	// SemanticBuild is the on-demand snapshot build switch for the semantic map;
	// nil (absent) means enabled (default true). A pointer distinguishes
	// "not set" from an explicit false.
	SemanticBuild *bool `yaml:"semantic_build"`
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
	opts := semanticOptions{Enabled: cfg.Search.Semantic, BuildOnDemand: true}
	if cfg.Search.SemanticBuild != nil {
		opts.BuildOnDemand = *cfg.Search.SemanticBuild
	}
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
	hashes := make(map[string]string)
	for _, e := range refresh.Manifest.EntriesSorted() {
		hashes[e.ID] = e.Hash
	}
	// Same single build path as the CLI and the on-demand map build.
	sem, _, err := semantic.BuildSnapshot(context.Background(), s.root, model, ix.SemanticSections(), hashes, mdindex.ManifestVersion)
	if err != nil {
		slog.Warn("sdtviewer: semantic index unavailable, serving lexical-only", "err", err, "model", model)
		s.setSemanticIndex(nil)
		return
	}
	s.setSemanticIndex(sem)
}

// ensureSemanticSnapshot builds the vector snapshot on demand when the semantic
// map needs it and none exists yet. It is a no-op when on-demand building is
// disabled, when a snapshot already exists, or when the search index is
// unavailable. Concurrent callers are serialized (single-flight) so the model is
// loaded and the corpus embedded at most once. It returns a non-empty warning
// string when a build was attempted and failed, so the map can show the reason
// instead of a bare empty state.
func (s *server) ensureSemanticSnapshot(ctx context.Context) string {
	if !s.semOpts.BuildOnDemand {
		return ""
	}
	if !semantic.LoadSnapshot(s.root).Empty() {
		return ""
	}
	s.semBuildMu.Lock()
	defer s.semBuildMu.Unlock()
	// Re-check under the lock: a queued caller may have built it already.
	if !semantic.LoadSnapshot(s.root).Empty() {
		return ""
	}
	ix := s.index()
	if ix == nil {
		return ""
	}
	refresh, err := mdindex.EnsureFresh(s.root)
	if err != nil {
		return "semantic build unavailable"
	}
	model := s.semOpts.Model
	if model == "" {
		model = semantic.ModelBase8M
	}
	hashes := make(map[string]string)
	for _, e := range refresh.Manifest.EntriesSorted() {
		hashes[e.ID] = e.Hash
	}
	if _, _, err := semantic.BuildSnapshot(ctx, s.root, model, ix.SemanticSections(), hashes, mdindex.ManifestVersion); err != nil {
		slog.Warn("sdtviewer: on-demand semantic build failed", "err", err, "model", model)
		return "semantic build failed"
	}
	return ""
}
