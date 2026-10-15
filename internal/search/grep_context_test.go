//go:build !wasm

package search

import (
	"os"
	"path/filepath"
	"testing"
)

// contextTree writes one file and returns its root, so each case states the
// content it reasons about rather than sharing a fixture it must remember.
func contextTree(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// contextMatches runs a grep for "M" with the given context counts.
func contextMatches(t *testing.T, content string, before, after, max int) GrepResults {
	t.Helper()
	res, err := Grep(GrepOptions{Root: contextTree(t, content), Pattern: "M", Before: before, After: after, Max: max})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestGrepAfterContextLines is the fallback this feature removes: `grep -A n`.
func TestGrepAfterContextLines(t *testing.T) {
	res := contextMatches(t, "a\nM1\nb\nc\nd\nM2\ne\n", 0, 2, 0)
	if len(res.Matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(res.Matches))
	}
	first := res.Matches[0]
	if first.Line != 2 || first.Text != "M1" {
		t.Fatalf("first match = line %d %q", first.Line, first.Text)
	}
	if first.Before != nil {
		t.Errorf("no before was asked for, got %v", first.Before)
	}
	want := []GrepContext{{Line: 3, Text: "b"}, {Line: 4, Text: "c"}}
	if len(first.After) != len(want) {
		t.Fatalf("after = %v, want %v", first.After, want)
	}
	for i, w := range want {
		if first.After[i] != w {
			t.Errorf("after[%d] = %v, want %v", i, first.After[i], w)
		}
	}
	// The second match is two lines from the end: it gets one line, not two.
	if len(res.Matches[1].After) != 1 || res.Matches[1].After[0].Line != 7 {
		t.Errorf("last match after = %v, want one line (7)", res.Matches[1].After)
	}
}

// TestGrepBeforeContextLines is the other half of the shorthand group.
func TestGrepBeforeContextLines(t *testing.T) {
	res := contextMatches(t, "a\nb\nc\nM\nd\n", 2, 0, 0)
	if len(res.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(res.Matches))
	}
	got := res.Matches[0].Before
	want := []GrepContext{{Line: 2, Text: "b"}, {Line: 3, Text: "c"}}
	if len(got) != len(want) {
		t.Fatalf("before = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("before[%d] = %v, want %v", i, got[i], w)
		}
	}
	if res.Matches[0].After != nil {
		t.Errorf("no after was asked for, got %v", res.Matches[0].After)
	}
}

// TestGrepContextAtFileEdges pins the two boundaries: a match on the first line
// has no before, and a match on the last line has no after — in particular the
// split's trailing empty element must not appear as a phantom line.
func TestGrepContextAtFileEdges(t *testing.T) {
	res := contextMatches(t, "M first\nmid\nM last\n", 3, 3, 0)
	if len(res.Matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(res.Matches))
	}
	if res.Matches[0].Before != nil {
		t.Errorf("a match on line 1 has no before, got %v", res.Matches[0].Before)
	}
	last := res.Matches[1]
	if len(last.After) != 0 {
		t.Errorf("a match on the last line has no after, got %v", last.After)
	}
	// Both preceding lines are context, including the one that is also a match:
	// context is "the lines near it", not "the lines that are not matches".
	want := []GrepContext{{Line: 1, Text: "M first"}, {Line: 2, Text: "mid"}}
	if len(last.Before) != len(want) {
		t.Fatalf("last match before = %v, want %v", last.Before, want)
	}
	for i, w := range want {
		if last.Before[i] != w {
			t.Errorf("before[%d] = %v, want %v", i, last.Before[i], w)
		}
	}
	// A file ending without a newline is one line shorter, not one longer.
	res = contextMatches(t, "M only", 0, 3, 0)
	if len(res.Matches) != 1 || len(res.Matches[0].After) != 0 {
		t.Errorf("a single-line file has no context after it: %v", res.Matches)
	}
}

// TestGrepContextLinesAreNotMatches is the composition rule: a context line does
// not satisfy the pattern, does not become a row, and does not increment a
// counter. The counters are what make a zero answer unambiguous.
func TestGrepContextLinesAreNotMatches(t *testing.T) {
	// "a" never matches, so both lines around the single match are context and
	// the counts must reflect one match in one file.
	res := contextMatches(t, "a\na\nM\na\na\n", 2, 2, 0)
	if len(res.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(res.Matches))
	}
	if res.FilesMatched != 1 || res.FilesSearched != 1 {
		t.Errorf("counters = matched %d searched %d, want 1 and 1", res.FilesMatched, res.FilesSearched)
	}
	if got := len(res.Matches[0].Before) + len(res.Matches[0].After); got != 4 {
		t.Errorf("context lines = %d, want 4", got)
	}
}

// TestGrepContextDoesNotExtendTheCap is the cap arithmetic the phase turns on: a
// match that trips the cap is not reported, and it brings no context with it —
// otherwise a cap of one would still print the neighbours of every later match.
func TestGrepContextDoesNotExtendTheCap(t *testing.T) {
	res := contextMatches(t, "M1\na\nM2\nb\nM3\n", 0, 1, 1)
	if len(res.Matches) != 1 {
		t.Fatalf("matches = %d, want the cap of 1", len(res.Matches))
	}
	if len(res.Matches[0].After) != 1 || res.Matches[0].After[0].Text != "a" {
		t.Fatalf("the reported match kept its own context: %v", res.Matches[0].After)
	}
	for _, m := range res.Matches {
		for _, c := range m.After {
			if c.Text == "b" {
				t.Errorf("context from a match past the cap leaked into the result: %v", c)
			}
		}
	}
	if !res.Truncated {
		t.Error("a cap that stopped the scan must set truncated")
	}
}

// TestGrepWithoutContextOmitsTheFields keeps the default output byte-identical
// to what it was before context existed.
func TestGrepWithoutContextOmitsTheFields(t *testing.T) {
	res := contextMatches(t, "a\nM\nb\n", 0, 0, 0)
	if len(res.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(res.Matches))
	}
	if res.Matches[0].Before != nil || res.Matches[0].After != nil {
		t.Errorf("no context was asked for: before %v after %v", res.Matches[0].Before, res.Matches[0].After)
	}
}

// TestGrepEmptyPatternDoesNotMatchAPhantomLine guards the line-splitting fix: a
// file ending in a newline has no line after it, so an empty pattern counts the
// lines the file actually has.
func TestGrepEmptyPatternDoesNotMatchAPhantomLine(t *testing.T) {
	res, err := Grep(GrepOptions{Root: contextTree(t, "a\nb\nc\n"), Pattern: "^", Max: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 3 {
		lines := make([]int, 0, len(res.Matches))
		for _, m := range res.Matches {
			lines = append(lines, m.Line)
		}
		t.Fatalf("a three-line file matches three lines, got %d at %v", len(res.Matches), lines)
	}
	// A file with no trailing newline has the same three lines.
	res, err = Grep(GrepOptions{Root: contextTree(t, "a\nb\nc"), Pattern: "^", Max: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 3 {
		t.Errorf("the trailing newline must not change the line count, got %d", len(res.Matches))
	}
}
