package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

func frontierPage(id, title, rels string) string {
	return "---\nkind: wiki\nid: " + id + "\ntitle: " + title +
		"\ntype: concept\nstatus: active\nsummary: s\ntags:\n  - x\n" + rels +
		"---\n## Summary\nbody\n"
}

func TestWikiFrontierScoresAndOrder(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "hub.md"), frontierPage("hub", "Hub",
		"relations:\n  refers_to:\n    - \"[[leaf|Leaf]]\"\n    - \"[[other|Other]]\"\n"))
	writeTestFile(t, filepath.Join(dir, "leaf.md"), frontierPage("leaf", "Leaf", ""))
	writeTestFile(t, filepath.Join(dir, "other.md"), frontierPage("other", "Other", ""))

	bld, _, err := contextwiki.LoadWiki(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries := wikiFrontier(bld, time.Now())
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	// hub: 2 outbound, 0 inbound -> highest. leaf and other are each referenced
	// by hub (0 outbound, 1 inbound) -> equal negative score, tie-broken by id.
	if entries[0].ID != "hub" || entries[0].OutDegree != 2 || entries[0].InDegree != 0 {
		t.Errorf("top = %+v, want hub 2/0", entries[0])
	}
	if entries[1].ID != "leaf" || entries[2].ID != "other" {
		t.Errorf("tie order = %s,%s, want leaf,other", entries[1].ID, entries[2].ID)
	}
	if entries[1].Score >= 0 {
		t.Errorf("referenced page score = %v, want negative", entries[1].Score)
	}
}

func TestWikiFrontierEmptyAndTieBreak(t *testing.T) {
	dir := t.TempDir()
	if bld, _, err := contextwiki.LoadWiki(dir); err == nil {
		if got := wikiFrontier(bld, time.Now()); len(got) != 0 {
			t.Errorf("empty wiki frontier = %d, want 0", len(got))
		}
	}

	dir2 := t.TempDir()
	writeTestFile(t, filepath.Join(dir2, "a.md"), frontierPage("a", "A", ""))
	writeTestFile(t, filepath.Join(dir2, "b.md"), frontierPage("b", "B", ""))
	bld, _, err := contextwiki.LoadWiki(dir2)
	if err != nil {
		t.Fatal(err)
	}
	got := wikiFrontier(bld, time.Now())
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("equal scores must tie-break by id: %+v", got)
	}
}
