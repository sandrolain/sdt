package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDoc writes one corpus document under root, creating its directories.
func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// fixture builds a corpus holding one edge of every family: a plan derived from
// an analysis (derivation + lifecycle), a plan correlated to a note
// (correlation), a note whose link leaves the corpus (dropped), and a wiki pair
// joined by a frontmatter relation and a typed body link.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeDoc(t, root, "context/analysis/20260101-000000-a.md", `---
kind: analysis
uid: u-a
title: Analysis A
status: active
summary: the parent analysis
---
## Body
`)

	writeDoc(t, root, "context/plan/20260101-000000-p.md", `---
kind: plan
uid: u-p
title: Plan P
status: active
summary: derived from A
analysis_id: u-a
sources:
  - analysis/20260101-000000-a.md
links:
  - notes/20260101-000000-n.md
---
## Body
`)

	writeDoc(t, root, "context/notes/20260101-000000-n.md", `---
kind: notes
uid: u-n
title: Note N
summary: a note with a reference outside the corpus
links:
  - refs/converted/outside.md
---
## Body
`)

	writeDoc(t, root, "context/wiki/topic/alpha.md", `---
kind: wiki
id: topic/alpha
type: concept
title: Alpha
status: active
summary: the source page
relations:
  part_of:
    - "[[topic/beta|Beta]]"
---
[[refines::topic/beta]]
`)

	writeDoc(t, root, "context/wiki/topic/beta.md", `---
kind: wiki
id: topic/beta
type: concept
title: Beta
status: active
summary: the target page
---
## Body
`)

	// A relation whose target does not exist: dropped, never half-resolved.
	writeDoc(t, root, "context/wiki/topic/gamma.md", `---
kind: wiki
id: topic/gamma
type: concept
title: Gamma
status: active
summary: a page pointing nowhere
relations:
  depends_on:
    - "[[topic/missing]]"
---
## Body
`)

	return root
}

func build(t *testing.T) *Graph {
	t.Helper()
	g, err := Build(fixture(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return g
}

// TestFamiliesHaveAnEdge is the binding test: every family the package declares
// is produced by the fixture corpus, so a family cannot be added without a
// fixture and cannot silently disappear from a surface.
func TestFamiliesHaveAnEdge(t *testing.T) {
	g := build(t)
	for _, family := range Families {
		found := false
		for _, e := range g.Edges() {
			if e.Family == family {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("family %q produced no edge in the fixture", family)
		}
	}
	if got, want := len(g.Edges()), 5; got != want {
		t.Errorf("edges = %d, want %d: %v", got, want, g.Edges())
	}
}

func TestDerivationAndCorrelation(t *testing.T) {
	g := build(t)
	plan := "context/plan/20260101-000000-p.md"
	analysis := "context/analysis/20260101-000000-a.md"
	note := "context/notes/20260101-000000-n.md"

	der := g.Outbound(plan, "")
	if len(der) != 3 { // derivation + correlation + lifecycle
		t.Fatalf("outbound plan = %d edges, want 3: %v", len(der), der)
	}

	if got := g.Outbound(plan, ""); !hasEdge(got, plan, analysis, Derivation, "") {
		t.Errorf("plan has no derivation edge to the analysis: %v", got)
	}
	if got := g.Outbound(plan, ""); !hasEdge(got, plan, note, Correlation, "") {
		t.Errorf("plan has no correlation edge to the note: %v", got)
	}
	if got := g.Inbound(analysis, ""); !hasEdge(got, plan, analysis, Lifecycle, "") {
		t.Errorf("analysis has no inbound lifecycle edge: %v", got)
	}
	// A reference outside the corpus is not an edge.
	for _, e := range g.Edges() {
		if e.From == note {
			t.Errorf("note emits an edge for a refs/ target: %v", e)
		}
	}
}

func TestWikiAndBodyFamilies(t *testing.T) {
	g := build(t)
	alpha := "context/wiki/topic/alpha.md"
	beta := "context/wiki/topic/beta.md"

	out := g.Outbound(alpha, "")
	if len(out) != 2 {
		t.Fatalf("outbound alpha = %d, want 2 (wiki + body): %v", len(out), out)
	}
	if !hasEdge(out, alpha, beta, Wiki, "part_of") {
		t.Errorf("alpha has no wiki edge: %v", out)
	}
	if !hasEdge(out, alpha, beta, Body, "refines") {
		t.Errorf("alpha has no body edge: %v", out)
	}
	if got := g.Inbound(beta, "part_of"); len(got) != 1 || got[0].Label != "Beta" {
		t.Errorf("inbound beta part_of = %v, want one edge labelled Beta", got)
	}
	if got := g.Outbound("context/wiki/topic/gamma.md", ""); len(got) != 0 {
		t.Errorf("an unresolved target produced an edge: %v", got)
	}
}

func TestVerbFilterAndUnknownVerb(t *testing.T) {
	g := build(t)
	alpha := "context/wiki/topic/alpha.md"
	if got := g.Outbound(alpha, "part_of"); len(got) != 1 {
		t.Errorf("verb filter both = %v, want 1", got)
	}
	if got := g.Outbound(alpha, "conflicts_with"); len(got) != 0 {
		t.Errorf("unknown verb = %v, want none", got)
	}
	if got := g.Outbound("context/notes/missing.md", ""); len(got) != 0 {
		t.Errorf("missing document = %v, want none", got)
	}
}

func TestRefFormsAndAccessors(t *testing.T) {
	g := build(t)
	analysis := "context/analysis/20260101-000000-a.md"
	for _, ref := range []string{analysis, "analysis/20260101-000000-a.md", "./" + analysis} {
		if got := g.Inbound(ref, ""); len(got) == 0 {
			t.Errorf("Inbound(%q) = none, want the plan edges", ref)
		}
	}
	if got := g.Len(); got != len(g.Edges()) {
		t.Errorf("Len() = %d, Edges() = %d", got, len(g.Edges()))
	}
	// The returned slice is a copy: mutating it cannot corrupt the graph.
	edges := g.Edges()
	edges[0].To = "tampered"
	if g.Edges()[0].To == "tampered" {
		t.Error("Edges() returned the internal slice")
	}
}

func TestNormalizeRef(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"analysis/x.md", "context/analysis/x.md"},
		{"./analysis/x.md", "context/analysis/x.md"},
		{"context/analysis/x.md", "context/analysis/x.md"},
		{"/context/analysis/x.md", "context/analysis/x.md"},
		{"refs/converted/x.md", ""},
		{"context/refs/x.md", ""},
		{"tmp/x.md", ""},
		{"trash/x.md", ""},
		{"scripts/x.py", ""},
		{"sdtdocs/x.md", ""},
	}
	for _, c := range cases {
		if got := NormalizeRef(c.in); got != c.want {
			t.Errorf("NormalizeRef(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWikiRef(t *testing.T) {
	if got, want := WikiRef(nil), ""; got != want {
		t.Errorf("WikiRef(nil) = %q, want %q", got, want)
	}
}

func TestBuildWithoutWiki(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/notes/20260101-000000-n.md", `---
kind: notes
title: N
status: active
summary: no wiki here
---
## Body
`)
	g, err := Build(root)
	if err != nil {
		t.Fatalf("Build without a wiki: %v", err)
	}
	if got := g.Len(); got != 0 {
		t.Errorf("edges = %d, want 0", got)
	}
}

// hasEdge reports whether the slice holds an edge with these endpoints, family
// and verb.
func hasEdge(edges []Edge, from, to string, family Family, verb string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Family == family && e.Verb == verb {
			return true
		}
	}
	return false
}
