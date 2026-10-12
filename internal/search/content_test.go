//go:build !wasm

package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeRoot creates a fixture tree: tracked files at several depths, an ignored
// file and directory, a nested .gitignore scoping a sibling, a binary file and
// a dependency tree.
func treeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "*.log\nbuild/\nnode_modules/\n")
	write("keep.md", "alpha needle\nsecond line\n")
	write("nested/deep/target.md", "no match here\n")
	write("notes/ignored.log", "needle in an ignored file\n")
	write("build/artifact.txt", "needle in a build artifact\n")
	write("sub/.gitignore", "local-only.md\n")
	write("sub/local-only.md", "needle in a nested ignore\n")
	write("sub/visible.md", "needle visible\n")
	write("node_modules/pkg/index.js", "needle in a dependency\n")
	write("binary.dat", "needle\x00\x01binary\x00")
	return root
}

func TestFindMatchesBasenameAtAnyDepth(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*.md"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"keep.md", "nested/deep/target.md", "sub/visible.md"}
	assertPaths(t, got, want)
}

func TestFindMatchesWholePathWhenPatternHasSeparator(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "nested/deep/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{"nested/deep/target.md"})
}

func TestFindDoubleStarSpansDirectories(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "**/deep/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	assertPaths(t, got, []string{"nested/deep/target.md"})
}

func TestFindHonoursGitignoreByDefault(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		switch {
		case strings.HasSuffix(r.Path, ".log"),
			strings.HasPrefix(r.Path, "build/"),
			r.Path == "sub/local-only.md",
			strings.HasPrefix(r.Path, "node_modules/"):
			t.Errorf("%s is ignored by .gitignore but was returned", r.Path)
		}
	}
	if !hasFindPath(got, "keep.md") {
		t.Errorf("tracked file missing: %+v", got)
	}
}

func TestFindNoIgnoreReturnsRawTree(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*", NoIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"notes/ignored.log", "build/artifact.txt", "sub/local-only.md"} {
		if !hasFindPath(got, want) {
			t.Errorf("--no-ignore missing %s: %+v", want, got)
		}
	}
}

func TestFindRejectsNonDirectoryRoot(t *testing.T) {
	if _, err := Find(FindOptions{Root: filepath.Join(treeRoot(t), "keep.md"), Pattern: "*"}); err == nil {
		t.Fatal("want an error for a file root")
	}
}

func TestFindIncludesDirectoriesOnRequest(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*", IncludeDirs: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range got {
		if r.Path == "sub" {
			found = true
			if !r.IsDir {
				t.Error("sub reported as a file")
			}
		}
	}
	if !found {
		t.Errorf("directories not included: %+v", got)
	}
}

func TestGrepFindsMatchesWithPathAndLine(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("want 2 matches (keep.md, sub/visible.md), got %d: %+v", len(res.Matches), res.Matches)
	}
	first := res.Matches[0]
	if first.Path != "keep.md" || first.Line != 1 {
		t.Errorf("first match = %+v, want keep.md:1", first)
	}
	if res.FilesMatched != 2 {
		t.Errorf("filesMatched = %d, want 2", res.FilesMatched)
	}
	if res.Truncated {
		t.Error("a two-match scan must not report truncation")
	}
}

func TestGrepSkipsIgnoredBinaryAndDependencyFiles(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Matches {
		switch {
		case strings.HasSuffix(m.Path, ".log"),
			strings.HasPrefix(m.Path, "build/"),
			m.Path == "sub/local-only.md",
			strings.HasPrefix(m.Path, "node_modules/"),
			m.Path == "binary.dat":
			t.Errorf("%s must not be searched", m.Path)
		}
	}
}

func TestGrepNoIgnoreReachesIgnoredFiles(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", NoIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, m := range res.Matches {
		paths[m.Path] = true
	}
	for _, want := range []string{"notes/ignored.log", "build/artifact.txt", "sub/local-only.md"} {
		if !paths[want] {
			t.Errorf("--no-ignore missing %s: %+v", want, paths)
		}
	}
}

func TestGrepIncludeExcludeScoping(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", Include: "sub/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Path != "sub/visible.md" {
		t.Fatalf("include glob did not scope the search: %+v", res.Matches)
	}
	res, err = Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", Exclude: "sub/*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Matches {
		if strings.HasPrefix(m.Path, "sub/") {
			t.Errorf("exclude glob did not drop %s", m.Path)
		}
	}
}

func TestGrepRespectsTotalCap(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "e", Max: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("total cap not applied: %+v", res.Matches)
	}
	if !res.Truncated {
		t.Error("a capped scan must report truncation")
	}
	// Reaching the cap exactly on the last match of the tree is not truncation:
	// nothing was withheld.
	res, err = Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", Max: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 || res.Truncated {
		t.Errorf("a cap equal to the match count must not report truncation: %+v", res)
	}
}

func TestGrepRespectsPerFileCap(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "e", MaxCountPerFile: 1, Max: -1})
	if err != nil {
		t.Fatal(err)
	}
	perFile := map[string]int{}
	for _, m := range res.Matches {
		perFile[m.Path]++
	}
	for path, n := range perFile {
		if n > 1 {
			t.Errorf("%s reported %d matches, want at most 1", path, n)
		}
	}
}

func TestGrepCaseInsensitive(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "NEEDLE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("a case-sensitive search matched: %+v", res.Matches)
	}
	res, err = Grep(GrepOptions{Root: treeRoot(t), Pattern: "NEEDLE", CaseInsensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) == 0 {
		t.Fatal("case-insensitive search found nothing")
	}
}

func TestGrepReportsInvalidPattern(t *testing.T) {
	if _, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "([a-z"}); err == nil {
		t.Fatal("want an error for an invalid regular expression")
	}
}

func TestGlobRegexpRules(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.md", "a.md", true},
		{"*.md", "deep/a.md", true},
		{"*.md", "a.txt", false},
		{"deep/*.md", "deep/a.md", true},
		{"deep/*.md", "deeper/a.md", false},
		{"**/*.md", "deep/a.md", true},
		{"**/deep/*.md", "x/deep/a.md", true},
		{"a?.md", "ab.md", true},
		{"a?.md", "abc.md", false},
		{"a+b.md", "a+b.md", true},
		{"a+b.md", "ab.md", false},
	}
	for _, c := range cases {
		re, err := globRegexp(c.pattern)
		if err != nil {
			t.Fatalf("globRegexp(%q): %v", c.pattern, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q against %q = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func assertPaths(t *testing.T, got []FindResult, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d paths %+v, want %d %v", len(got), got, len(want), want)
	}
	for i, w := range want {
		if got[i].Path != w {
			t.Errorf("path %d = %q, want %q (all: %+v)", i, got[i].Path, w, got)
		}
	}
}

func hasFindPath(got []FindResult, path string) bool {
	for _, r := range got {
		if r.Path == path {
			return true
		}
	}
	return false
}
