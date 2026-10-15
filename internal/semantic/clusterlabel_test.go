package semantic

import "testing"

func TestClusterLabelsNamesItsPlurality(t *testing.T) {
	groups := map[int][]ClusterMember{
		0: {
			{Path: "context/notes/b.md", Kind: "notes", Title: "Beta"},
			{Path: "context/notes/a.md", Kind: "notes", Title: "Alpha notes"},
			{Path: "context/notes/c.md", Kind: "notes", Title: "Gamma"},
		},
		1: {
			{Path: "context/decisions/0001-a.md", Kind: "decision", Title: "Auth choice"},
			{Path: "context/architecture/x.md", Kind: "architecture", Title: "Viewer"},
			{Path: "context/decisions/0002-b.md", Kind: "decision", Title: "Storage choice"},
		},
		2: {{Path: "context/plans/p.md"}},
	}
	got := ClusterLabels(groups)
	if want := "notes · Beta"; got[0] != want {
		t.Errorf("cluster 0 = %q, want %q (shortest title of the dominant kind, ties by path)", got[0], want)
	}
	// Two of three is above the dominant share: named, not numbered.
	if want := "decision · Auth choice"; got[1] != want {
		t.Errorf("cluster 1 = %q, want %q", got[1], want)
	}
	// A member with no kind is `unknown`, and one member is that kind alone.
	if want := "unknown"; got[2] != want {
		t.Errorf("cluster 2 = %q, want %q", got[2], want)
	}
}

func TestClusterLabelsNumbersAMixedCluster(t *testing.T) {
	groups := map[int][]ClusterMember{
		3: {
			{Path: "context/notes/a.md", Kind: "notes", Title: "Alpha"},
			{Path: "context/decisions/b.md", Kind: "decision", Title: "Beta"},
			{Path: "context/wiki/c.md", Kind: "wiki", Title: "Gamma"},
			{Path: "context/analysis/d.md", Kind: "analysis", Title: "Delta"},
		},
	}
	// No kind reaches the dominant share, so the label names the plurality and its
	// index instead of claiming a coherence the grouping does not have.
	if got, want := ClusterLabels(groups)[3], "analysis 3"; got != want {
		t.Errorf("mixed cluster = %q, want %q", got, want)
	}
}

func TestClusterLabelsIsDeterministicAndBounded(t *testing.T) {
	long := "A title long enough to need the bound applied to it somewhere in the middle"
	groups := map[int][]ClusterMember{
		0: {
			{Path: "b.md", Kind: "analysis", Title: long},
			{Path: "a.md", Kind: "analysis", Title: long},
		},
	}
	first := ClusterLabels(groups)
	for i := 0; i < 5; i++ {
		if got := ClusterLabels(groups); got[0] != first[0] {
			t.Fatalf("run %d = %q, want %q", i, got[0], first[0])
		}
	}
	label := first[0]
	if len([]rune(label)) > labelTitleRunes+len("analysis")+len(" · ") {
		t.Errorf("label %q is not bounded", label)
	}
	if !containsRune(label, '…') {
		t.Errorf("label %q should carry the ellipsis the bound adds", label)
	}
}

func TestClusterLabelsOmitsEmptyGroups(t *testing.T) {
	if got := ClusterLabels(map[int][]ClusterMember{2: {}}); len(got) != 0 {
		t.Errorf("labels = %v, want none for an empty group", got)
	}
	if got := ClusterLabels(nil); len(got) != 0 {
		t.Errorf("labels = %v, want none for no groups", got)
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
