package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listFixture builds a small tree: two files at the root, one in a subdirectory
// and one deeper, plus a context/tmp scratch file that the default scope prunes.
func listFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"sub/deep", "context/tmp"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"a.txt":               "a",
		"b.go":                "package b",
		"sub/c.txt":           "cc",
		"sub/deep/d.txt":      "ddd",
		"context/tmp/scratch": "s",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func listEntries(t *testing.T, opts ListOptions) ListResult {
	t.Helper()
	res, err := List(opts)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return res
}

// TestListIsOneLevelByDefault proves the listing answers "what is here", not
// "what is anywhere": the root's entries only, directories first.
func TestListIsOneLevelByDefault(t *testing.T) {
	root := listFixture(t)
	res := listEntries(t, ListOptions{Root: root})
	var paths []string
	for _, e := range res.Entries {
		paths = append(paths, e.Path)
	}
	want := []string{"context", "sub", "a.txt", "b.go"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v", paths, want)
	}
}

// TestListDepthReachesOneLevelDown proves --depth 2 includes the grandchildren
// of the root but stops there.
func TestListDepthReachesOneLevelDown(t *testing.T) {
	root := listFixture(t)
	res := listEntries(t, ListOptions{Root: root, Depth: 2})
	seen := map[string]bool{}
	for _, e := range res.Entries {
		seen[e.Path] = true
	}
	for _, want := range []string{"sub/c.txt", "sub/deep"} {
		if !seen[want] {
			t.Errorf("depth 2 misses %q (got %v)", want, seen)
		}
	}
	if seen["sub/deep/d.txt"] {
		t.Error("depth 2 must not reach three levels down")
	}
}

// TestListStatFillsSizeAndModTime proves --stat is the difference between a
// path list and a listing: sizes and times come from the entry itself.
func TestListStatFillsSizeAndModTime(t *testing.T) {
	root := listFixture(t)
	res := listEntries(t, ListOptions{Root: root, Depth: 2, Stat: true})
	byPath := map[string]ListEntry{}
	for _, e := range res.Entries {
		byPath[e.Path] = e
	}
	if got := byPath["sub/c.txt"].Size; got != 2 {
		t.Errorf("size of sub/c.txt = %d, want 2", got)
	}
	if byPath["sub/c.txt"].Modified.IsZero() {
		t.Error("--stat must fill the modification time")
	}
	plain := listEntries(t, ListOptions{Root: root, Depth: 2})
	for _, e := range plain.Entries {
		if !e.Modified.IsZero() || e.Size != 0 {
			t.Errorf("without --stat %q carries stat data", e.Path)
		}
	}
}

// TestListLimitMarksTruncated proves a capped listing says so: a silent cap
// reads as "that is everything".
func TestListLimitMarksTruncated(t *testing.T) {
	root := listFixture(t)
	res := listEntries(t, ListOptions{Root: root, Max: 2})
	if len(res.Entries) != 2 || res.Count != 2 || !res.Truncated {
		t.Errorf("limited listing = %+v", res)
	}
}

// TestListPrunesTheScratchAndRefs proves the listing shows the same scope as
// find: context/tmp is not part of the tree an agent lists.
func TestListPrunesTheScratchAndRefs(t *testing.T) {
	root := listFixture(t)
	res := listEntries(t, ListOptions{Root: root, Depth: 3})
	for _, e := range res.Entries {
		if strings.HasPrefix(e.Path, "context/tmp") {
			t.Errorf("context/tmp must be pruned, got %q", e.Path)
		}
	}
}

// TestListRefusesAFile proves a file is answered with the reason, never with an
// empty listing: the two would be indistinguishable to a caller.
func TestListRefusesAFile(t *testing.T) {
	root := listFixture(t)
	if _, err := List(ListOptions{Root: filepath.Join(root, "a.txt")}); err == nil {
		t.Fatal("a file must be refused")
	}
}

// TestListMissingRootIsAnError proves a typo fails rather than listing nothing.
func TestListMissingRootIsAnError(t *testing.T) {
	if _, err := List(ListOptions{Root: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("a missing directory must be an error")
	}
}
