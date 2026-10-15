//go:build !wasm

package search

import (
	"strings"
	"testing"
)

// TestTrashIsPrunedInBothRefModes pins the exclusion the discard area needs, and
// the half of it that is easy to lose: `--refs` widens the walk to the captured
// clones and must leave the trash pruned, exactly as it leaves tmp pruned.
// Opting into a third-party clone is not an opt-in to what someone threw away.
func TestTrashIsPrunedInBothRefModes(t *testing.T) {
	root := presenceTree(t, map[string]string{
		"live.md":                 "needle\n",
		"context/tmp/scratch.md":  "needle\n",
		"context/trash/gone.md":   "needle\n",
		"context/refs/clone/x.md": "needle\n",
	})

	for _, tc := range []struct {
		name      string
		skip      []string
		wantPaths []string
	}{
		{"default", nil, []string{"live.md"}},
		{"refs", DefaultSkipPathsNoRefs, []string{"context/refs/clone/x.md", "live.md"}},
	} {
		opts := GrepOptions{Root: root, Pattern: "needle", Max: 0}
		if tc.skip != nil {
			opts.SkipPaths = tc.skip
		}
		res, err := Grep(opts)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var got []string
		for _, m := range res.Matches {
			if len(got) == 0 || got[len(got)-1] != m.Path {
				got = append(got, m.Path)
			}
		}
		if strings.Join(got, ",") != strings.Join(tc.wantPaths, ",") {
			t.Errorf("%s: searched %v, want %v", tc.name, got, tc.wantPaths)
		}
	}
}

// TestFindPrunesTheTrash keeps the two search verbs on the same scope: a
// discarded file is not a path the tree reports.
func TestFindPrunesTheTrash(t *testing.T) {
	root := presenceTree(t, map[string]string{
		"live.md":               "x\n",
		"context/trash/gone.md": "x\n",
	})
	res, err := Find(FindOptions{Root: root, Pattern: "*.md"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if strings.Contains(r.Path, "trash") {
			t.Errorf("find reported a discarded file: %v", res)
		}
	}
	if len(res) != 1 {
		t.Errorf("find returned %v, want only the live file", res)
	}
}
