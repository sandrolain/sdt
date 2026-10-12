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

// contentTree creates a working tree with a tracked file, an ignored file and a
// binary file, and points the process at it.
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
	write(".gitignore", "*.log\n")
	write("src/keep.txt", "alpha\nneedle line\n")
	write("src/other.txt", "nothing here\n")
	write("src/drop.log", "needle ignored\n")
	write("src/blob.bin", "needle\x00binary\n")
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
	if len(res.Matches) != 1 {
		t.Fatalf("want the single tracked match, got %+v", res.Matches)
	}
	if res.Matches[0].Path != "src/keep.txt" || res.Matches[0].Line != 2 {
		t.Errorf("match = %+v, want src/keep.txt:2", res.Matches[0])
	}
	if res.Truncated {
		t.Error("a one-match scan must not report truncation")
	}
}

func TestGrepCommandSkipsIgnoredAndBinaryFiles(t *testing.T) {
	contentTree(t)
	for _, args := range [][]string{
		{"needle"},
		{"needle", "--no-ignore"},
	} {
		res := runGrep(t, args...)
		for _, m := range res.Matches {
			if strings.HasSuffix(m.Path, ".log") && len(args) == 1 {
				t.Errorf("ignored file %s was searched", m.Path)
			}
			if strings.HasSuffix(m.Path, ".bin") {
				t.Errorf("binary file %s was searched", m.Path)
			}
		}
	}
}

func TestGrepCommandNoIgnoreReachesIgnoredFile(t *testing.T) {
	contentTree(t)
	res := runGrep(t, "needle", "--no-ignore")
	if !hasGrepPath(res.Matches, "src/drop.log") {
		t.Errorf("--no-ignore did not reach the ignored file: %+v", res.Matches)
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
	if len(res.Matches) != 1 {
		t.Errorf("max-count not applied: %+v", res.Matches)
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

func TestFindCommandOmitsGlobListsEveryTrackedFile(t *testing.T) {
	contentTree(t)
	res := runFind(t)
	if len(res) != 4 { // .gitignore, src/keep.txt, src/other.txt, src/blob.bin
		t.Fatalf("want every tracked file, got %+v", res)
	}
	if hasFindPathIn(res, "src/drop.log") {
		t.Errorf("ignored file listed: %+v", res)
	}
}

func TestFindCommandNoIgnoreAndDirsFlags(t *testing.T) {
	contentTree(t)
	res := runFind(t, "--no-ignore")
	if !hasFindPathIn(res, "src/drop.log") {
		t.Errorf("--no-ignore did not reach the ignored file: %+v", res)
	}
	res = runFind(t, "--dirs")
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
