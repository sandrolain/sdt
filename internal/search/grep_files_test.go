//go:build !wasm

package search

import (
	"os"
	"path/filepath"
	"testing"
)

// presenceTree writes several files and returns the root, so a case can state
// which ones carry a match.
func presenceTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// presenceGrep runs the mode under test.
func presenceGrep(t *testing.T, root string, max int) GrepResults {
	t.Helper()
	res, err := Grep(GrepOptions{Root: root, Pattern: "needle", FilesWithMatches: true, Max: max})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestGrepFilesWithMatchesListsEachFileOnce is the fallback this mode removes:
// `grep -rl | sort -u` turned one process into two, and this returns the same
// answer from one scan.
func TestGrepFilesWithMatchesListsEachFileOnce(t *testing.T) {
	root := presenceTree(t, map[string]string{
		"a.txt":     "needle\nneedle\nneedle\n",
		"b.txt":     "nothing\n",
		"c/deep.md": "x\nneedle\ny\n",
	})
	res := presenceGrep(t, root, 0)
	want := []string{"a.txt", "c/deep.md"}
	if len(res.Paths) != len(want) {
		t.Fatalf("paths = %v, want %v", res.Paths, want)
	}
	for i, w := range want {
		if res.Paths[i] != w {
			t.Errorf("paths[%d] = %q, want %q", i, res.Paths[i], w)
		}
	}
	// Three matches in one file is still one path, and the answer is not
	// repeated for them.
	for _, p := range res.Paths {
		if p == "a.txt" {
			count := 0
			for _, q := range res.Paths {
				if q == "a.txt" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("a.txt appears %d times", count)
			}
		}
	}
}

// TestGrepFilesWithMatchesLeavesMatchesEmpty pins the shape: the answer is in
// Paths, so a caller does not read around placeholder rows to find it.
func TestGrepFilesWithMatchesLeavesMatchesEmpty(t *testing.T) {
	res := presenceGrep(t, presenceTree(t, map[string]string{"a.txt": "needle\n"}), 0)
	if len(res.Matches) != 0 {
		t.Errorf("matches = %v, want none in this mode", res.Matches)
	}
	if res.FilesMatched != 1 || res.FilesSearched != 1 {
		t.Errorf("counters = matched %d searched %d, want 1 and 1", res.FilesMatched, res.FilesSearched)
	}
}

// TestGrepFilesWithMatchesCountsPathsAgainstTheCap is the cap decision: --limit
// bounds the list of files. Counting suppressed rows instead would let one noisy
// file spend the whole budget on a single path.
func TestGrepFilesWithMatchesCountsPathsAgainstTheCap(t *testing.T) {
	root := presenceTree(t, map[string]string{
		"a.txt": "needle\nneedle\nneedle\nneedle\n",
		"b.txt": "needle\n",
		"c.txt": "needle\n",
	})
	res := presenceGrep(t, root, 2)
	if len(res.Paths) != 2 {
		t.Fatalf("paths = %v, want the cap of 2", res.Paths)
	}
	if !res.Truncated {
		t.Error("a cap that stopped the scan must set truncated")
	}
	if res.FilesMatched != 2 {
		t.Errorf("filesMatched = %d, want the 2 reported paths", res.FilesMatched)
	}
}

// TestGrepFilesWithMatchesZeroAnswerIsUnambiguous keeps the property that made
// `grep | echo sentinel` unnecessary: no paths with the counters present says
// the tree was searched, not that the scan failed.
func TestGrepFilesWithMatchesZeroAnswerIsUnambiguous(t *testing.T) {
	res := presenceGrep(t, presenceTree(t, map[string]string{"a.txt": "nothing\n", "b.txt": "nothing\n"}), 0)
	if len(res.Paths) != 0 {
		t.Fatalf("paths = %v, want none", res.Paths)
	}
	if res.FilesSearched != 2 {
		t.Errorf("filesSearched = %d, want the 2 files that were read", res.FilesSearched)
	}
	if res.Truncated {
		t.Error("an exhaustive empty answer is not truncated")
	}
}
