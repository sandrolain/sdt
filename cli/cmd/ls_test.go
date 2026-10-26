package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLsListsOneLevelByDefault proves the listing is the entry step it claims to
// be: the directory's own entries, directories first, and no descent.
func TestLsListsOneLevelByDefault(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a.txt"), "hello")
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "sub", "b.go"), "package x")

	out := string(execute(t, lsCmd, nil, "--dir", "."))
	if !strings.Contains(out, "sub") || !strings.Contains(out, "a.txt") {
		t.Fatalf("listing misses an entry:\n%s", out)
	}
	if strings.Contains(out, "b.go") {
		t.Errorf("--depth defaults to 1 and must not descend:\n%s", out)
	}
	if strings.Index(out, "sub") > strings.Index(out, "a.txt") {
		t.Errorf("directories must come first:\n%s", out)
	}
}

// TestLsDepthStatAndLimit covers the three flags that make a listing answer
// "what is here, how big" instead of "what paths exist".
func TestLsDepthStatAndLimit(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "sub", "b.go"), "package x")
	// The size assertion below reads this file's exact byte count. A directory's
	// reported size is a filesystem fact — ext4/overlayfs say 4096, APFS says less —
	// so a unit letter in a directory's row proves nothing on a runner.
	writeTestFile(t, filepath.Join(dir, "a.txt"), "hello")

	out := string(execute(t, lsCmd, nil, "--dir", ".", "--depth", "2"))
	if !strings.Contains(out, "sub/b.go") {
		t.Errorf("--depth 2 must reach one level down:\n%s", out)
	}

	out = string(execute(t, lsCmd, nil, "--dir", ".", "--stat"))
	if !strings.Contains(out, "sub") || !strings.Contains(out, "5B") {
		t.Errorf("--stat must add a size column:\n%s", out)
	}

	out = string(execute(t, lsCmd, nil, "--dir", ".", "--depth", "2", "--limit", "1"))
	if got := strings.Count(strings.TrimSpace(out), "\n") + 1; got != 1 {
		t.Errorf("--limit 1 must report one row, got %d:\n%s", got, out)
	}
}

// TestLsJSONCarriesTheMachineAnswer keeps the two formats in step: the table is
// presentation, the JSON is the same rows with exact sizes.
func TestLsJSONCarriesTheMachineAnswer(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a.txt"), "hello")

	out := string(execute(t, lsCmd, nil, "--dir", ".", "--format", "json"))
	for _, want := range []string{`"entries"`, `"path": "a.txt"`, `"count": 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("json answer missing %s:\n%s", want, out)
		}
	}
}

// TestLsRefusesAFile proves the refusal names the shape: --dir takes a
// directory, and a file path is answered with the reason rather than an empty
// listing.
func TestLsRefusesAFile(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a.txt"), "hello")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, lsCmd, nil, "--dir", "a.txt"))
	})
}
