//go:build !wasm

package search

import (
	"testing"
)

// countGrep runs the count mode over a fixture tree.
func countGrep(t *testing.T, files map[string]string, maxCount int) GrepResults {
	t.Helper()
	res, err := Grep(GrepOptions{
		Root:            presenceTree(t, files),
		Pattern:         "needle",
		CountOnly:       true,
		MaxCountPerFile: maxCount,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestGrepCountOnlyCountsWithoutKeepingRows is the fallback this mode removes:
// the number of matching lines, with no rows to parse around.
func TestGrepCountOnlyCountsWithoutKeepingRows(t *testing.T) {
	res := countGrep(t, map[string]string{
		"a.txt": "needle\nx\nneedle\n",
		"b.txt": "needle\n",
		"c.txt": "nothing\n",
	}, 0)
	if res.Count == nil {
		t.Fatal("a count was asked for and none was reported")
	}
	if *res.Count != 3 {
		t.Errorf("count = %d, want 3", *res.Count)
	}
	if len(res.Matches) != 0 {
		t.Errorf("count mode keeps no rows, got %v", res.Matches)
	}
	if res.FilesMatched != 2 || res.FilesSearched != 3 {
		t.Errorf("counters = matched %d searched %d, want 2 and 3", res.FilesMatched, res.FilesSearched)
	}
}

// TestGrepCountZeroIsARealAnswer keeps a zero count distinguishable from "no
// count was requested": the field is present and holds zero.
func TestGrepCountZeroIsARealAnswer(t *testing.T) {
	res := countGrep(t, map[string]string{"a.txt": "nothing\n"}, 0)
	if res.Count == nil {
		t.Fatal("a zero count must still be reported, not omitted")
	}
	if *res.Count != 0 {
		t.Errorf("count = %d, want 0", *res.Count)
	}
	if res.Truncated {
		t.Error("an exhaustive zero is not truncated")
	}
}

// TestGrepCountWithoutTheModeOmitsTheField is the other side: a normal grep must
// not carry a count that reads as "nothing matched".
func TestGrepCountWithoutTheModeOmitsTheField(t *testing.T) {
	res, err := Grep(GrepOptions{Root: presenceTree(t, map[string]string{"a.txt": "needle\n"}), Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != nil {
		t.Errorf("count mode was not asked for, got %d", *res.Count)
	}
	if len(res.Matches) != 1 {
		t.Errorf("matches = %v, want the row", res.Matches)
	}
}

// TestGrepCountPerFileCapClipsAndSaysSo pins the GNU-compatible half: -m stops
// the scan of a file, so the count is clipped, and a clipped count is never
// presented as a total.
func TestGrepCountPerFileCapClipsAndSaysSo(t *testing.T) {
	res := countGrep(t, map[string]string{
		"a.txt": "needle\nneedle\nneedle\n",
		"b.txt": "needle\n",
	}, 1)
	if res.Count == nil || *res.Count != 2 {
		t.Fatalf("count = %v, want one per file with --max-count 1", res.Count)
	}
	if !res.Truncated {
		t.Error("a clipped count must be marked truncated")
	}
}

// TestSplitLinesIsTheOneOwnerOfTheLineRule pins the rule the count, the slice
// and the scanner all read: a trailing newline terminates the last line.
func TestSplitLinesIsTheOneOwnerOfTheLineRule(t *testing.T) {
	cases := []struct {
		text  string
		lines int
	}{
		{"", 0},
		{"\n", 1},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"a\n\n", 2},
		{"a\r\nb\r\n", 2},
	}
	for _, tc := range cases {
		if got := len(SplitLines(tc.text)); got != tc.lines {
			t.Errorf("SplitLines(%q) = %d lines, want %d", tc.text, got, tc.lines)
		}
	}
}
