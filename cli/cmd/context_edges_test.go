package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

// edgeFixture is the contract both implementations assert: the resolved parent
// and children of every document under the fixture corpus.
// web/src/lib/edges.test.ts reads the same file, so the Go resolver and the
// viewer payload cannot disagree about who derives from whom.
type edgeFixture struct {
	Edges map[string]struct {
		Parent   string   `json:"parent"`
		Children []string `json:"children"`
	} `json:"edges"`
}

// lifecycleEdgeFixtureDir is the shared fixture, relative to this package; its
// `context/` subdirectory is the miniature corpus the resolver reads, mirroring
// the production layout.
const lifecycleEdgeFixtureDir = "testdata/lifecycle-edges"

func loadEdgeFixture(t *testing.T) edgeFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(lifecycleEdgeFixtureDir, "expected-edges.json"))
	if err != nil {
		t.Fatalf("read edge fixture: %v", err)
	}
	var f edgeFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse edge fixture: %v", err)
	}
	if len(f.Edges) == 0 {
		t.Fatal("edge fixture is empty")
	}
	return f
}

func TestCtxrelMatchesTheSharedEdgeFixture(t *testing.T) {
	edges, err := ctxrel.Load(filepath.Join(lifecycleEdgeFixtureDir, "context"))
	if err != nil {
		t.Fatalf("ctxrel.Load: %v", err)
	}
	fixture := loadEdgeFixture(t)
	for ref, want := range fixture.Edges {
		if got := edges.ParentOf(ref); got != want.Parent {
			t.Errorf("ParentOf(%q) = %q, want %q", ref, got, want.Parent)
		}
		got := edges.ChildrenOf(ref)
		if len(got) != len(want.Children) {
			t.Errorf("ChildrenOf(%q) = %v, want %v", ref, got, want.Children)
			continue
		}
		for i, child := range want.Children {
			if got[i] != child {
				t.Errorf("ChildrenOf(%q)[%d] = %q, want %q", ref, i, got[i], child)
			}
		}
	}
}

// A `sources` or `links` citation is not an edge, and the fixture is what keeps
// that true: it contains exactly those cases.
func TestEdgeFixtureCoversTheAmbiguousCases(t *testing.T) {
	fixture := loadEdgeFixture(t)
	cases := map[string]string{
		// cited in a plan's sources, but not its parent
		"context/analysis/sibling.md": "",
		// links a plan, declares no plan_id
		"context/tasks/linked.md": "",
		// cites another plan, declares plan_id
		"context/tasks/citing.md": "context/plan/normal.md",
		// analysis_id that resolves to nothing
		"context/plan/dangling.md": "",
		// no relation at all
		"context/tasks/loose.md": "",
	}
	for ref, want := range cases {
		entry, ok := fixture.Edges[ref]
		if !ok {
			t.Fatalf("fixture is missing %s", ref)
		}
		if entry.Parent != want {
			t.Errorf("fixture %s parent = %q, want %q", ref, entry.Parent, want)
		}
	}
}
