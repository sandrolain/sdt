// Package graph is the one reader of the corpus edge model: it turns every
// relation a document records into a typed edge, in both directions, so no
// consumer re-parses frontmatter to answer "what points at this".
//
// Five families exist, and the distinction between them is the schema's, not an
// accident:
//
//   - derivation — a document's `sources` list (where it comes from);
//   - correlation — its `links` list (what it is about);
//   - wiki — a wiki page's frontmatter `relations` (the typed knowledge graph);
//   - body — a typed body link, `[[verb::target]]` (the graph's second family);
//   - lifecycle — the typed parent chain, resolved by internal/ctxrel.
//
// The corpus frontmatter is the only place an edge is written. This package
// stores nothing: Build derives the graph from the shared mdindex manifest, the
// ctxrel resolver and the wiki builder, all of which rebuild from the markdown.
//
// An edge names a document, or it is not an edge: a reference outside the corpus
// (`refs/…`), a target excluded from it, and a wiki relation whose target does
// not resolve are dropped here, exactly as the viewer's backlink index and graph
// dropped them before. `Label` carries the wikilink label when one was written.
package graph

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/mdindex"
)

// Family is the kind of relation an edge expresses.
type Family string

const (
	// Derivation is a `sources` edge: the document this one derives from.
	Derivation Family = "derivation"
	// Correlation is a `links` edge: a document this one is about.
	Correlation Family = "correlation"
	// Wiki is a frontmatter `relations` edge: the typed knowledge graph.
	Wiki Family = "wiki"
	// Body is a typed body link, `[[verb::target]]`.
	Body Family = "body"
	// Lifecycle is the typed parent chain (`analysis_id`, `plan_id`).
	Lifecycle Family = "lifecycle"
)

// Families lists every family once. The binding test iterates it, so a family
// added without a fixture fails here rather than disappearing from a surface.
var Families = []Family{Derivation, Correlation, Wiki, Body, Lifecycle}

// Edge is one relation between two documents, both named by their
// project-relative reference (`context/…`). Verb is the relation verb for the
// wiki and body families (the closed ten of the wiki schema, or the typed-link
// verb) and empty for the three list families. Label is the wikilink label, or
// the target's title when none was written.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Family Family `json:"family"`
	Verb   string `json:"verb,omitempty"`
	Label  string `json:"label,omitempty"`
}

// Graph is the derived edge set, indexed for both directions. Build it once per
// process and read it many times; it is immutable after Build.
type Graph struct {
	edges []Edge
	out   map[string][]Edge
	in    map[string][]Edge
}

// Build derives the graph for the project rooted at projectRoot.
func Build(projectRoot string) (*Graph, error) {
	refresh, err := mdindex.EnsureFresh(projectRoot)
	if err != nil {
		return nil, err
	}
	life, err := ctxrel.Load(filepath.Join(projectRoot, corpus.ContextDir))
	if err != nil {
		return nil, err
	}
	wiki, err := loadWiki(filepath.Join(projectRoot, corpus.ContextDir, "wiki"))
	if err != nil {
		return nil, err
	}
	var manifest *mdindex.Manifest
	if refresh != nil {
		manifest = refresh.Manifest
	}
	return From(manifest, wiki, life), nil
}

// From is the pure constructor: a manifest, a wiki builder and the resolved
// lifecycle edges in, a Graph out. It exists so a process that already holds the
// manifest at rebuild time — the viewer, which watches the corpus — derives the
// edge set without re-scanning. A nil argument contributes no edge of its kind,
// so a wiki-only graph is `From(nil, builder, nil)`.
func From(m *mdindex.Manifest, wiki *contextwiki.Builder, life *ctxrel.Edges) *Graph {
	b := builder{g: &Graph{out: map[string][]Edge{}, in: map[string][]Edge{}}}
	if m != nil {
		b.addManifest(m, life)
	}
	b.addWiki(wiki)
	b.finish()
	return b.g
}

// loadWiki loads the wiki builder, treating a missing wiki directory as empty: a
// project with no wiki has no wiki edges, which is not a failure.
func loadWiki(dir string) (*contextwiki.Builder, error) {
	bld, _, err := contextwiki.LoadWiki(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &contextwiki.Builder{}, nil
		}
		return nil, err
	}
	if bld == nil {
		bld = &contextwiki.Builder{}
	}
	return bld, nil
}

// Outbound returns the edges leaving ref, optionally filtered to one verb. A verb
// that is empty or absent returns the whole set; the result is a copy the caller
// may keep or reorder.
func (g *Graph) Outbound(ref, verb string) []Edge {
	return filter(g.index(g.out, ref), verb)
}

// Inbound returns the edges arriving at ref, optionally filtered to one verb.
func (g *Graph) Inbound(ref, verb string) []Edge {
	return filter(g.index(g.in, ref), verb)
}

// Len is the number of distinct edges in the graph.
func (g *Graph) Len() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

// Edges returns every edge, sorted by source, family, verb and target.
func (g *Graph) Edges() []Edge {
	if g == nil {
		return nil
	}
	out := make([]Edge, len(g.edges))
	copy(out, g.edges)
	return out
}

func (g *Graph) index(m map[string][]Edge, ref string) []Edge {
	if g == nil {
		return nil
	}
	return m[NormalizeRef(ref)]
}

func filter(edges []Edge, verb string) []Edge {
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if verb == "" || e.Verb == verb {
			out = append(out, e)
		}
	}
	return out
}

// builder accumulates edges while Build walks its three sources.
type builder struct {
	g    *Graph
	seen map[string]bool
}

func (b *builder) add(e Edge) {
	from, to := NormalizeRef(e.From), NormalizeRef(e.To)
	if from == "" || to == "" || from == to {
		return
	}
	e.From, e.To = from, to
	key := from + "\x00" + to + "\x00" + string(e.Family) + "\x00" + e.Verb
	if b.seen == nil {
		b.seen = map[string]bool{}
	}
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	b.g.edges = append(b.g.edges, e)
}

func (b *builder) addManifest(m *mdindex.Manifest, life *ctxrel.Edges) {
	for id, e := range m.Entries {
		if e == nil {
			continue
		}
		for _, ref := range e.Values["sources"] {
			b.add(Edge{From: id, To: ref, Family: Derivation})
		}
		for _, ref := range e.Values["links"] {
			b.add(Edge{From: id, To: ref, Family: Correlation})
		}
		if parent := life.ParentOf(id); parent != "" {
			b.add(Edge{From: id, To: parent, Family: Lifecycle})
		}
	}
}

// addWiki adds the wiki's two families: the frontmatter relations and the typed
// body links. A target that does not resolve to a page is dropped, which is the
// graph's behaviour today.
func (b *builder) addWiki(wiki *contextwiki.Builder) {
	if wiki == nil {
		return
	}
	for _, p := range wiki.Pages {
		from := WikiRef(p)
		if from == "" {
			continue
		}
		for verb, targets := range p.Relations {
			for _, raw := range targets {
				m := contextwiki.LinkRegexp.FindStringSubmatch(raw)
				if m == nil {
					continue
				}
				tgt := wiki.Resolve(strings.TrimSpace(m[1]))
				if tgt == nil {
					continue
				}
				label := strings.TrimSpace(m[2])
				if label == "" {
					label = tgt.Title
				}
				b.add(Edge{From: from, To: WikiRef(tgt), Family: Wiki, Verb: verb, Label: label})
			}
		}
		for _, l := range p.Links {
			if l.Verb == "" {
				continue
			}
			tgt := wiki.Resolve(l.Target)
			if tgt == nil {
				continue
			}
			b.add(Edge{From: from, To: WikiRef(tgt), Family: Body, Verb: l.Verb, Label: tgt.Title})
		}
	}
}

// finish sorts the edge list and indexes both directions.
func (b *builder) finish() {
	g := b.g
	sort.SliceStable(g.edges, func(i, j int) bool {
		a, z := g.edges[i], g.edges[j]
		if a.From != z.From {
			return a.From < z.From
		}
		if a.Family != z.Family {
			return a.Family < z.Family
		}
		if a.Verb != z.Verb {
			return a.Verb < z.Verb
		}
		return a.To < z.To
	})
	for _, e := range g.edges {
		g.out[e.From] = append(g.out[e.From], e)
		g.in[e.To] = append(g.in[e.To], e)
	}
}

// WikiRef maps a wiki page to its project-relative file reference. Mapping the
// file, not the frontmatter id, keeps every endpoint in one identity space: a
// wiki page is a document like any other in the corpus.
func WikiRef(p *contextwiki.Page) string {
	if p == nil || p.FileID == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(corpus.ContextDir, "wiki", p.FileID+contextwiki.MarkdownExt))
}

// NormalizeRef reduces a corpus reference to the one form an edge endpoint
// carries: project-relative with the `context/` prefix. It accepts the forms the
// corpus writes (`analysis/x.md`, `./analysis/x.md`, `context/analysis/x.md`) and
// returns "" for an empty reference, a target outside the corpus (`refs/…`) and
// an excluded path (`tmp/`, `trash/`, `scripts/`, `sdtdocs/`) — a document the
// corpus does not serve is not a node.
func NormalizeRef(ref string) string {
	p := strings.TrimSpace(filepath.ToSlash(ref))
	if p == "" {
		return ""
	}
	p = strings.TrimPrefix(strings.TrimPrefix(p, "./"), "/")
	if !strings.HasPrefix(p, corpus.ContextDir+"/") {
		// `sources`/`links` are written corpus-internal (`analysis/x.md`), so a
		// bare reference is read as one; refs/ lives outside the served corpus.
		if strings.HasPrefix(p, "refs/") {
			return ""
		}
		p = corpus.ContextDir + "/" + p
	}
	if corpus.ExcludedPath(p) {
		return ""
	}
	return p
}
