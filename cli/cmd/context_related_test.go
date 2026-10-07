package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/semantic"
)

func TestRelatedNeighborsEnrichAndOrder(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: alpha\n---\nbody a\n")
	writeCtxDoc(t, "context/notes/b.md", "---\nkind: notes\nsummary: beta\n---\nbody b\n")
	writeCtxDoc(t, "context/notes/c.md", "---\nkind: research\nstatus: draft\nsummary: gamma\n---\nbody c\n")
	snap := &semantic.Snapshot{SectionVectors: map[string][]float32{
		"context/notes/a.md#body": {1, 0},
		"context/notes/b.md#body": {0.9, 0.1},
		"context/notes/c.md#body": {0, 1},
	}}
	hits := relatedNeighbors(snap, "context/notes/a.md", 5)
	if len(hits) != 2 || hits[0].Path != "context/notes/b.md" || hits[0].Summary != "beta" {
		t.Fatalf("hits = %#v, want b enriched first", hits)
	}
	if hits[1].Kind != "research" || hits[1].Status != "draft" {
		t.Fatalf("second hit not enriched: %#v", hits[1])
	}
}

func TestRelatedDegradesWithoutSnapshot(t *testing.T) {
	runInTempDir(t)
	out := string(execute(t, contextRelatedCmd, nil, "context/notes/a.md"))
	if !strings.Contains(out, "no semantic snapshot") {
		t.Fatalf("want the degradation message, got %q", out)
	}
}

func TestRelatedDocPath(t *testing.T) {
	cases := map[string]string{
		"notes/x.md":              "context/notes/x.md",
		"context/notes/x":         "context/notes/x.md",
		"context/notes/x.md#body": "context/notes/x.md",
	}
	for in, want := range cases {
		if got := relatedDocPath(in); got != want {
			t.Errorf("relatedDocPath(%q) = %q, want %q", in, got, want)
		}
	}
}
