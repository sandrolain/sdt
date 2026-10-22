//go:build !wasm

package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeRoot creates a fixture tree: files at several depths, a `.gitignore` in
// the managed sdt block naming a file, a directory and a dependency tree, a
// nested `.gitignore` beside a sibling, and a binary file.
//
// The ignore files are here to be *ignored*: nothing in this tree is excluded
// because one of them names it. The dependency tree is still pruned, because
// that is the tool's own scope.
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
	write(".gitignore", "# sdt:start\n*.log\nbuild/\nnode_modules/\n# sdt:end\n")
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
	want := []string{"keep.md", "nested/deep/target.md", "sub/local-only.md", "sub/visible.md"}
	assertPaths(t, got, want)
}

// TestFindAnchorsPathPatternsToTheWholePath pins the anchoring: a pattern
// carrying a slash matches the path, not a suffix of it. Unanchored, a glob
// matched any tail, so a corpus glob was answered by a test fixture nested
// deeper in the tree and the --limit filled up before a real document appeared.
func TestFindAnchorsPathPatternsToTheWholePath(t *testing.T) {
	root := treeRoot(t)
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("deep/nested/notes/deep.md", "far away\n")
	write("notes/real.md", "a real note\n")

	got, err := Find(FindOptions{Root: root, Pattern: "notes/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindPath(got, "notes/real.md") {
		t.Errorf("notes/*.md did not match its own directory: %+v", got)
	}
	if hasFindPath(got, "deep/nested/notes/deep.md") {
		t.Errorf("notes/*.md matched a nested path it should not: %+v", got)
	}

	// ** is the explicit way to say "at any depth", and it still works.
	got, err = Find(FindOptions{Root: root, Pattern: "**/notes/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindPath(got, "deep/nested/notes/deep.md") {
		t.Errorf("**/notes/*.md did not span directories: %+v", got)
	}
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

// TestFindIgnoresGitignoreFiles pins the removal: an ignore file names nothing
// that stays out of the walk. Only skipDirs prunes, and it prunes by directory
// name.
func TestFindIgnoresGitignoreFiles(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"keep.md", "notes/ignored.log", "build/artifact.txt", "sub/local-only.md",
	} {
		if !hasFindPath(got, want) {
			t.Errorf("%s is named by an ignore file and must still be found: %+v", want, got)
		}
	}
	for _, r := range got {
		if strings.HasPrefix(r.Path, "node_modules/") {
			t.Errorf("dependency tree %s was walked; skipDirs must prune it", r.Path)
		}
	}
}

func TestFindReturnsTheRawTree(t *testing.T) {
	got, err := Find(FindOptions{Root: treeRoot(t), Pattern: "*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"notes/ignored.log", "build/artifact.txt", "sub/local-only.md"} {
		if !hasFindPath(got, want) {
			t.Errorf("the raw tree is missing %s: %+v", want, got)
		}
	}
}

func TestFindRejectsNonDirectoryRoot(t *testing.T) {
	if _, err := Find(FindOptions{Root: filepath.Join(treeRoot(t), "keep.md"), Pattern: "*"}); err == nil {
		t.Fatal("want an error for a file root")
	}
}

// TestGrepRejectsNonDirectoryRoot is the O1 gate: the walk used to drop the
// only entry (a root-relative "." is outside the root), so a file passed as
// --dir produced an empty result with exit 0 — the same answer as "no match".
// The refusal names the path and a directory root still scans.
func TestGrepRejectsNonDirectoryRoot(t *testing.T) {
	root := treeRoot(t)
	_, err := Grep(GrepOptions{Root: filepath.Join(root, "keep.md"), Pattern: "needle"})
	if err == nil {
		t.Fatal("want an error for a file root")
	}
	if !strings.Contains(err.Error(), "is not a directory") || !strings.Contains(err.Error(), "keep.md") {
		t.Errorf("the refusal must name the path and the reason: %v", err)
	}
	res, err := Grep(GrepOptions{Root: root, Pattern: "needle"})
	if err != nil {
		t.Fatalf("a directory root must still scan: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Error("the directory scan found nothing")
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
	// Five files carry the needle; three of them are named by an ignore file and
	// are searched all the same, in path order.
	want := []string{
		"build/artifact.txt", "keep.md", "notes/ignored.log",
		"sub/local-only.md", "sub/visible.md",
	}
	if len(res.Matches) != len(want) {
		t.Fatalf("want %d matches %v, got %d: %+v", len(want), want, len(res.Matches), res.Matches)
	}
	for i, p := range want {
		if res.Matches[i].Path != p {
			t.Errorf("match %d = %q, want %q", i, res.Matches[i].Path, p)
		}
	}
	if res.Matches[1].Line != 1 {
		t.Errorf("keep.md match line = %d, want 1", res.Matches[1].Line)
	}
	if res.FilesMatched != len(want) {
		t.Errorf("filesMatched = %d, want %d", res.FilesMatched, len(want))
	}
	if res.Truncated {
		t.Error("a two-match scan must not report truncation")
	}
}

// TestGrepSkipsBinaryAndDependencyFilesOnly pins that the binary sniff and
// skipDirs still prune, while nothing prunes on account of an ignore file.
func TestGrepSkipsBinaryAndDependencyFilesOnly(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, m := range res.Matches {
		paths[m.Path] = true
	}
	for _, m := range res.Matches {
		switch {
		case strings.HasPrefix(m.Path, "node_modules/"),
			m.Path == "binary.dat":
			t.Errorf("%s must not be searched", m.Path)
		}
	}
	for _, want := range []string{"notes/ignored.log", "build/artifact.txt", "sub/local-only.md"} {
		if !paths[want] {
			t.Errorf("%s is named by an ignore file and must still be searched: %+v", want, paths)
		}
	}
}

func TestGrepIncludeExcludeScoping(t *testing.T) {
	res, err := Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", Include: "sub/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	// Both sub/ markdown files match now: the nested ignore file no longer hides
	// local-only.md, so --include has two hits to report.
	if len(res.Matches) != 2 {
		t.Fatalf("include glob did not scope the search: %+v", res.Matches)
	}
	for _, m := range res.Matches {
		if !strings.HasPrefix(m.Path, "sub/") {
			t.Errorf("include glob let %s through", m.Path)
		}
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
	// nothing was withheld. Five files carry "needle" now that the ignore file
	// excludes nothing, so the exact cap is five.
	res, err = Grep(GrepOptions{Root: treeRoot(t), Pattern: "needle", Max: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 5 || res.Truncated {
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

// scopeTree creates a project-root fixture with a declared scope: two pruned
// subtrees (one nested, to prove the subtree rule), a same-named directory
// outside the scope, and a source file that must survive.
func scopeTree(t *testing.T, declared []string) string {
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
	write("src/keep.go", "needle keep\n")
	write("dist/a.go", "needle dist\n")
	write("dist/assets/b.go", "needle nested under dist\n")
	write(".next/c.go", "needle next\n")
	write("distribution/d.go", "needle distribution is not dist\n")
	if len(declared) > 0 {
		body := "project: p\ngroup: g\nskipDirs:\n"
		for _, d := range declared {
			body += "  - " + d + "\n"
		}
		write(".sdt.yaml", body)
	}
	return root
}

func TestFindPrunesDeclaredScopeAtAnyDepth(t *testing.T) {
	root := scopeTree(t, []string{"dist", ".next"})
	got, err := Find(FindOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist", ".next"}, Pattern: "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"distribution/d.go", "src/keep.go"}
	assertPaths(t, got, want)
}

func TestFindDeclaredScopeIsProjectRootRelativeUnderDir(t *testing.T) {
	root := scopeTree(t, []string{"dist"})
	// The pattern is relative to the project root, so it prunes the same subtree
	// whether the walk starts at the root or deeper inside it.
	for _, r := range []string{root, filepath.Join(root, "src")} {
		got, err := Find(FindOptions{
			Root: r, ProjectRoot: root, SkipDirs: []string{"dist"}, Pattern: "*.go",
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, res := range got {
			// Exact prefix relative to the walk root, not a substring: the
			// same-prefix sibling `distribution/` must survive.
			if res.Path == "dist" || strings.HasPrefix(res.Path, "dist/") {
				t.Errorf("walk from %s returned the pruned subtree: %+v", r, got)
			}
		}
	}
}

func TestGrepDeclaredScopePrunesTheDescent(t *testing.T) {
	root := scopeTree(t, []string{"dist", ".next"})
	all, err := Grep(GrepOptions{Root: root, Pattern: "needle", Max: -1})
	if err != nil {
		t.Fatal(err)
	}
	declared, err := Grep(GrepOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist", ".next"},
		Pattern: "needle", Max: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The point is not that the matches shrink — the glob already excluded them
	// before. It is that the files are never opened.
	if all.FilesSearched-declared.FilesSearched != 3 {
		t.Errorf("pruned files = %d, want 3 (the descent must skip them, not filter them): %d vs %d",
			all.FilesSearched-declared.FilesSearched, all.FilesSearched, declared.FilesSearched)
	}
	for _, m := range declared.Matches {
		if m.Path == "dist" || strings.HasPrefix(m.Path, "dist/") || strings.HasPrefix(m.Path, ".next") {
			t.Errorf("a pruned file was searched: %+v", m)
		}
	}
}

func TestFindSubtreePatternCoversWhatIsBeneath(t *testing.T) {
	root := scopeTree(t, []string{"dist"})
	// `dist` must prune dist/assets/b.go without being written `dist/**`.
	got, err := Find(FindOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist"}, Pattern: "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, res := range got {
		if strings.HasPrefix(res.Path, "dist/") {
			t.Errorf("a bare directory pattern did not cover what is beneath it: %+v", got)
		}
	}
	// `dist/**` is accepted too, and must not leak to a same-prefix sibling.
	got, err = Find(FindOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist/**"}, Pattern: "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindPath(got, "distribution/d.go") {
		t.Errorf("dist/** pruned a same-prefix sibling: %+v", got)
	}
}

func TestFindWithoutADeclarationKeepsTheShippedDefaults(t *testing.T) {
	root := scopeTree(t, nil)
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "i.go"), []byte("needle dep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Find(FindOptions{Root: root, Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".next/c.go", "dist/a.go", "dist/assets/b.go", "distribution/d.go", "src/keep.go"}
	assertPaths(t, got, want)
}

func TestDeclaredScopeReplacesRatherThanExtendsTheDefaults(t *testing.T) {
	root := scopeTree(t, []string{"dist"})
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "i.go"), []byte("needle dep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Find(FindOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist"}, Pattern: "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The declaration replaced the five built-ins, so node_modules is searched
	// again: a project that says what its scope is is not silently given five
	// more it never asked about.
	if !hasFindPath(got, "node_modules/pkg/i.go") {
		t.Errorf("the declaration extended the defaults instead of replacing them: %+v", got)
	}
}

func TestPruneMatcherIgnoresPathsOutsideTheProject(t *testing.T) {
	m := newPruneMatcher("/project", []string{"dist"})
	if m.matches("/elsewhere/dist") {
		t.Error("a directory outside the project must not match a project-root pattern")
	}
	if m.matches("/project") {
		t.Error("the project root itself must never be pruned")
	}
	if !m.matches("/project/dist") {
		t.Error("a declared pattern did not match inside the project")
	}
	// One contract whatever the pattern: this path, or anything beneath it.
	if !m.matches("/project/dist/assets") {
		t.Error("the subtree rule does not cover what is beneath a bare pattern")
	}
	if m.matches("/project/distribution") {
		t.Error("a bare pattern matched a same-prefix sibling")
	}
}

func TestDeclaredScopeCannotUnpruneTheVCSResidues(t *testing.T) {
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
	write("src/keep.go", "needle keep\n")
	write("dist/a.go", "needle dist\n")
	write("node_modules/pkg/i.go", "needle dep\n")
	write(".git/config", "needle git config\n")
	write(".git/HEAD", "needle git head\n")

	got, err := Find(FindOptions{
		Root: root, ProjectRoot: root, SkipDirs: []string{"dist"}, Pattern: "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"node_modules/pkg/i.go", "src/keep.go"}
	assertPaths(t, got, want)
	for _, res := range got {
		if strings.HasPrefix(res.Path, ".git") {
			t.Errorf("a declaration un-pruned a VCS residue: %+v", got)
		}
	}
}
