//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/search"
)

// contentTree creates a working tree carrying the configuration `sdt agent
// init` produces — a `.gitignore` with the managed sdt block holding a
// directory entry — plus an ignored file, a binary file, a dependency tree and
// a corpus document, and points the process at it.
//
// The managed block is the shape that hid the defect: the search verbs must read
// none of it. Everything the ignore file names is expected in the results.
func contentTree(t *testing.T) string {
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
	write(".gitignore", "# sdt:start\n/context/\n*.log\n# sdt:end\n")
	write("src/keep.txt", "alpha\nneedle line\n")
	write("src/other.txt", "nothing here\n")
	write("src/drop.log", "needle ignored\n")
	write("src/blob.bin", "needle\x00binary\n")
	write("context/notes/a.md", "needle corpus\n")
	write("context/refs/extlib/README.md", "needle clone\n")
	write("context/tmp/scratch.txt", "needle scratch\n")
	write("refs/source-dir.md", "needle source refs dir\n")
	write("node_modules/pkg/x.txt", "needle dependency\n")
	t.Chdir(root)
	return root
}

func runGrep(t *testing.T, args ...string) search.GrepResults {
	t.Helper()
	out := execute(t, grepCmd, nil, append(args, "--format", "json")...)
	var res search.GrepResults
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode grep output: %v\n%s", err, out)
	}
	return res
}

func TestGrepCommandReportsMatches(t *testing.T) {
	contentTree(t)
	res := runGrep(t, "needle")
	// Every text file matches except the binary one; the dependency tree and the
	// corpus clones are pruned by the tool's own scope, not by git's rules.
	want := []string{"context/notes/a.md", "refs/source-dir.md", "src/drop.log", "src/keep.txt"}
	if len(res.Matches) != len(want) {
		t.Fatalf("matches = %+v, want %v", res.Matches, want)
	}
	for i, p := range want {
		if res.Matches[i].Path != p {
			t.Errorf("match %d = %q, want %q", i, res.Matches[i].Path, p)
		}
	}
	if res.Matches[0].Line != 1 {
		t.Errorf("first match line = %d, want 1", res.Matches[0].Line)
	}
	if res.Truncated {
		t.Error("a four-match scan must not report truncation")
	}
}

// TestGrepCommandIgnoresNoIgnoreFile pins the removal: nothing in the tree is
// excluded because an ignore file names it, so a file the .gitignore matches is
// searched like any other.
func TestGrepCommandIgnoresNoIgnoreFile(t *testing.T) {
	contentTree(t)
	res := runGrep(t, "needle")
	if !hasGrepPath(res.Matches, "src/drop.log") {
		t.Errorf("a file matched by *.log was not searched: %+v", res.Matches)
	}
	if !hasGrepPath(res.Matches, "context/notes/a.md") {
		t.Errorf("a file under the gitignored corpus was not searched: %+v", res.Matches)
	}
	for _, m := range res.Matches {
		if strings.HasSuffix(m.Path, ".bin") {
			t.Errorf("binary file %s was searched", m.Path)
		}
		if strings.HasPrefix(m.Path, "node_modules/") {
			t.Errorf("dependency tree %s was searched", m.Path)
		}
	}
}

// TestGrepCommandNoIgnoreFlagIsANoOp keeps the flag parseable while the raw tree
// is already the default; it must not change the result.
func TestGrepCommandNoIgnoreFlagIsANoOp(t *testing.T) {
	contentTree(t)
	plain := runGrep(t, "needle")
	flagged := runGrep(t, "needle", "--no-ignore")
	if len(flagged.Matches) != len(plain.Matches) {
		t.Errorf("--no-ignore changed the result: %+v vs %+v", flagged.Matches, plain.Matches)
	}
	if !hasGrepPath(flagged.Matches, "src/drop.log") {
		t.Errorf("--no-ignore lost an ignored file it should not affect: %+v", flagged.Matches)
	}
}

// TestContentSearchPrunesCorpusClonesByDefault pins the scope the tool owns now
// that it no longer inherits one from git: the captured clones and the scratch
// directory are skipped at the descent, and --refs brings the clones back.
func TestContentSearchPrunesCorpusClonesByDefault(t *testing.T) {
	contentTree(t)

	res := runGrep(t, "needle")
	for _, pruned := range []string{"context/refs/extlib/README.md", "context/tmp/scratch.txt"} {
		if hasGrepPath(res.Matches, pruned) {
			t.Errorf("%s was searched; the corpus scope should prune it", pruned)
		}
	}
	// A source directory that merely shares the name is not the corpus's, so it
	// stays searchable — the prune is by path, not by directory name.
	if !hasGrepPath(res.Matches, "refs/source-dir.md") {
		t.Errorf("a source directory named refs was pruned: %+v", res.Matches)
	}
	if !hasGrepPath(res.Matches, "context/notes/a.md") {
		t.Errorf("the corpus of work must stay searchable: %+v", res.Matches)
	}

	withRefs := runGrep(t, "needle", "--refs")
	if !hasGrepPath(withRefs.Matches, "context/refs/extlib/README.md") {
		t.Errorf("--refs did not reach the captured clones: %+v", withRefs.Matches)
	}
	if hasGrepPath(withRefs.Matches, "context/tmp/scratch.txt") {
		t.Errorf("--refs must not un-prune the scratch directory: %+v", withRefs.Matches)
	}
}

// TestFindPrunesCorpusClonesByDefault is the same scope on the path verb.
func TestFindPrunesCorpusClonesByDefault(t *testing.T) {
	contentTree(t)
	res := runFind(t)
	for _, pruned := range []string{"context/refs/extlib/README.md", "context/tmp/scratch.txt"} {
		if hasFindPathIn(res, pruned) {
			t.Errorf("%s was listed; the corpus scope should prune it", pruned)
		}
	}
	if !hasFindPathIn(res, "context/notes/a.md") {
		t.Errorf("the corpus of work must stay listed: %+v", res)
	}
	if !hasFindPathIn(res, "refs/source-dir.md") {
		t.Errorf("a source directory named refs was pruned: %+v", res)
	}
	if !hasFindPathIn(runFind(t, "--refs"), "context/refs/extlib/README.md") {
		t.Error("--refs did not list the captured clones")
	}
}

// TestContentSearchDirInsidePrunedTree pins that --dir wins over the prune: a
// caller who names the clones directory directly asked for it. Paths are
// reported relative to the search root, so the hit is extlib/README.md.
func TestContentSearchDirInsidePrunedTree(t *testing.T) {
	contentTree(t)
	res := runGrep(t, "needle", "--dir", "context/refs")
	if !hasGrepPath(res.Matches, "extlib/README.md") {
		t.Errorf("--dir into the pruned tree did not reach it: %+v", res.Matches)
	}
}

// The three reproductions recorded in analysis 20261014-063929, against the real
// corpus. They are the failures that shipped through a green gate, so they are
// pinned here against the actual project rather than only against a fixture.

// TestReproFindOnACorpusPath: `sdt find "context/architecture/stack.md"`
// returned an empty list because the corpus was invisible to the walk.
// projectRoot walks up from the package directory to the repository root, so a
// reproduction runs against the real corpus rather than a fixture that could
// drift from it.
func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func TestReproFindOnACorpusPath(t *testing.T) {
	root := projectRoot(t)
	t.Chdir(root)
	if _, err := os.Stat("context/architecture/stack.md"); err != nil {
		t.Skipf("corpus document missing: %v", err)
	}
	out := execute(t, findCmd, nil, "context/architecture/stack.md", "--format", "json")
	var res []search.FindResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	if len(res) != 1 || res[0].Path != "context/architecture/stack.md" {
		t.Errorf("a corpus path did not resolve: %+v", res)
	}
}

// TestReproGrepIncludeScopedToTheCorpus reported filesSearched: 0, which reads
// as a confident "the corpus does not discuss this".
func TestReproGrepIncludeScopedToTheCorpus(t *testing.T) {
	root := projectRoot(t)
	t.Chdir(root)
	if _, err := os.Stat("context/architecture"); err != nil {
		t.Skipf("corpus missing: %v", err)
	}
	res := runGrep(t, "status matrix", "--include", "context/architecture/*.md", "--limit", "5")
	if res.FilesSearched == 0 {
		t.Errorf("filesSearched = 0: an include scoped to the corpus searched nothing")
	}
	if len(res.Matches) == 0 {
		t.Errorf("no match for a string the corpus carries: %+v", res)
	}
	for _, m := range res.Matches {
		if !strings.HasPrefix(m.Path, "context/architecture/") {
			t.Errorf("include glob let %s through", m.Path)
		}
	}
}

// TestReproAPrintedPathRoundTrips: a path printed by find did not resolve when
// fed back as input, so an agent could not chain one call into the next.
func TestReproAPrintedPathRoundTrips(t *testing.T) {
	root := projectRoot(t)
	t.Chdir(root)
	out := execute(t, findCmd, nil, "context/architecture/*.md", "--format", "json", "--limit", "1")
	var first []search.FindResult
	if err := json.Unmarshal(out, &first); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	if len(first) == 0 {
		t.Skip("no corpus path to round-trip")
	}
	again := execute(t, findCmd, nil, first[0].Path, "--format", "json")
	var second []search.FindResult
	if err := json.Unmarshal(again, &second); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, again)
	}
	if len(second) != 1 || second[0].Path != first[0].Path {
		t.Errorf("printed path %q did not round-trip: %+v", first[0].Path, second)
	}
}

// TestReproNoTestFixtureStandsInForTheCorpus: the default view once answered a
// corpus glob with a cli/cmd/testdata fixture while hiding the real documents.
func TestReproNoTestFixtureStandsInForTheCorpus(t *testing.T) {
	root := projectRoot(t)
	t.Chdir(root)
	out := execute(t, findCmd, nil, "context/analysis/*.md", "--format", "json", "--limit", "10")
	var res []search.FindResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	if len(res) == 0 {
		t.Fatal("no real corpus analysis document found")
	}
	for _, r := range res {
		if strings.HasPrefix(r.Path, "cli/") {
			t.Errorf("a test fixture answered a corpus glob: %s", r.Path)
		}
	}
}

func TestGrepCommandIncludeExcludeFlags(t *testing.T) {
	contentTree(t)
	if err := os.WriteFile(filepath.Join("src", "second.txt"), []byte("needle two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runGrep(t, "needle", "--include", "src/second.txt")
	if len(res.Matches) != 1 || res.Matches[0].Path != "src/second.txt" {
		t.Fatalf("include did not scope the search: %+v", res.Matches)
	}
	res = runGrep(t, "needle", "--exclude", "src/second.txt")
	if hasGrepPath(res.Matches, "src/second.txt") {
		t.Errorf("exclude did not drop the file: %+v", res.Matches)
	}
}

func TestGrepCommandCapsAndFlags(t *testing.T) {
	contentTree(t)
	res := runGrep(t, "e", "--limit", "1")
	if len(res.Matches) != 1 || !res.Truncated {
		t.Errorf("limit not applied or not reported: %+v", res)
	}
	res = runGrep(t, "NEEDLE", "-i")
	if len(res.Matches) == 0 {
		t.Error("-i did not make the search case-insensitive")
	}
	res = runGrep(t, "needle", "--max-count", "1", "--limit", "0")
	// max-count caps per file, so one match per matching file still reports all
	// four; the point is that no file contributes more than one.
	if len(res.Matches) != 4 {
		t.Errorf("max-count per file = %d matches over 4 files: %+v", len(res.Matches), res.Matches)
	}
	perFile := map[string]int{}
	for _, m := range res.Matches {
		perFile[m.Path]++
	}
	for path, n := range perFile {
		if n > 1 {
			t.Errorf("--max-count 1 reported %d matches in %s", n, path)
		}
	}
}

func TestGrepCommandRejectsInvalidPattern(t *testing.T) {
	contentTree(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, grepCmd, nil, "([a-z", "--format", "json")
		return ""
	})
}

func TestFindCommandListsMatchingPaths(t *testing.T) {
	contentTree(t)
	out := execute(t, findCmd, nil, "*.txt", "--format", "json")
	var res []search.FindResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	if len(res) != 2 || res[0].Path != "src/keep.txt" || res[1].Path != "src/other.txt" {
		t.Fatalf("want the two tracked text files sorted, got %+v", res)
	}
}

func TestFindCommandOmitsGlobListsEveryFile(t *testing.T) {
	contentTree(t)
	res := runFind(t)
	// .gitignore, context/notes/a.md, refs/source-dir.md and the four src files.
	// The dependency tree is pruned by skipDirs and the corpus clones and scratch
	// directory by the corpus scope.
	want := []string{
		".gitignore", "context/notes/a.md", "refs/source-dir.md", "src/blob.bin",
		"src/drop.log", "src/keep.txt", "src/other.txt",
	}
	if len(res) != len(want) {
		t.Fatalf("find returned %d paths, want %d: %+v", len(res), len(want), res)
	}
	for i, p := range want {
		if res[i].Path != p {
			t.Errorf("path %d = %q, want %q", i, res[i].Path, p)
		}
	}
}

func TestFindCommandNoIgnoreIsANoOpAndDirsFlag(t *testing.T) {
	contentTree(t)
	plain := runFind(t)
	flagged := runFind(t, "--no-ignore")
	if len(flagged) != len(plain) {
		t.Errorf("--no-ignore changed the result: %+v vs %+v", flagged, plain)
	}
	if !hasFindPathIn(flagged, "src/drop.log") {
		t.Errorf("an ignored file is missing from the default listing: %+v", flagged)
	}
	res := runFind(t, "--dirs")
	if !hasFindPathIn(res, "src") {
		t.Errorf("--dirs did not include a directory: %+v", res)
	}
}

func TestFindCommandDirFlagScopesTheSearch(t *testing.T) {
	contentTree(t)
	out := execute(t, findCmd, nil, "*.txt", "--dir", "src", "--format", "json")
	var res []search.FindResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	// Paths are reported relative to the search root, so scoping --dir both
	// narrows the set and shortens the reported names.
	if len(res) != 2 || res[0].Path != "keep.txt" || res[1].Path != "other.txt" {
		t.Fatalf("--dir did not scope the search to its root: %+v", res)
	}
}

func TestGrepAndFindHelpCarryTheDeclaredShape(t *testing.T) {
	for _, cmd := range []*cobra.Command{grepCmd, findCmd} {
		for _, part := range []string{"WHEN —", "WHEN NOT —", "RETURNS —", "EXAMPLE —"} {
			if !strings.Contains(cmd.Long, part) {
				t.Errorf("%s help is missing the %s part", cmd.Use, part)
			}
		}
		if cmd.Example == "" {
			t.Errorf("%s carries no example", cmd.Use)
		}
	}
}

func runFind(t *testing.T, args ...string) []search.FindResult {
	t.Helper()
	out := execute(t, findCmd, nil, append(args, "--format", "json")...)
	var res []search.FindResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode find output: %v\n%s", err, out)
	}
	return res
}

func hasGrepPath(matches []search.GrepMatch, path string) bool {
	for _, m := range matches {
		if m.Path == path {
			return true
		}
	}
	return false
}

func hasFindPathIn(res []search.FindResult, path string) bool {
	for _, r := range res {
		if r.Path == path {
			return true
		}
	}
	return false
}
