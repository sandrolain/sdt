package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// readFixture writes a four-line file and returns its name.
func readFixture(t *testing.T) string {
	t.Helper()
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "alpha\nbravo\ncharlie\ndelta\n")
	return "f.txt"
}

// TestReadNumberedPrefixesTheFileNumbers proves the citation shape: the number
// is the file's own, not the selection's ordinal, so `--lines 3:4 --numbered`
// prints 3 and 4.
func TestReadNumberedPrefixesTheFileNumbers(t *testing.T) {
	file := readFixture(t)
	out := string(execute(t, fileReadCmd, nil, "-p", file, "--lines", "3:4", "--numbered"))
	if out != "3: charlie\n4: delta\n" {
		t.Errorf("numbered range = %q", out)
	}
}

// TestReadTailForm covers `--lines -N:`, the shape that replaced a second
// command: the last N lines, clamped when the file is shorter.
func TestReadTailForm(t *testing.T) {
	file := readFixture(t)
	out := string(execute(t, fileReadCmd, nil, "-p", file, "--lines", "-2:"))
	if out != "charlie\ndelta\n" {
		t.Errorf("tail form = %q", out)
	}
	out = string(execute(t, fileReadCmd, nil, "-p", file, "--lines", "-99:"))
	if out != "alpha\nbravo\ncharlie\ndelta\n" {
		t.Errorf("tail form must clamp, got %q", out)
	}
}

// TestReadFormatJSONAnswers proves the global flag is honoured instead of
// silently ignored: the answer is the documented object, not raw text.
func TestReadFormatJSONAnswers(t *testing.T) {
	file := readFixture(t)
	out := execute(t, fileReadCmd, nil, "-p", file, "--lines", "1:2", "--format", "json")
	var payload struct {
		Path     string   `json:"path"`
		Lines    []string `json:"lines"`
		Count    int      `json:"count"`
		Numbered bool     `json:"numbered"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json answer does not parse: %v\n%s", err, out)
	}
	if payload.Path != file || payload.Count != 2 || len(payload.Lines) != 2 || payload.Numbered {
		t.Errorf("json answer = %+v", payload)
	}
}

// TestReadRefusesNumberedWithSection proves the combination is refused rather
// than numbered from an arbitrary origin: a citation that points at the wrong
// line is worse than no citation.
func TestReadRefusesNumberedWithSection(t *testing.T) {
	file := readFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, fileReadCmd, nil, "-p", file, "--section", "Missing", "--numbered"))
	})
}

// TestGrepPathSearchesExactlyOneFile proves the one-file form reports the path
// the caller typed and ignores a same-named file elsewhere.
func TestGrepPathSearchesExactlyOneFile(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a", "f.txt"), "needle\n")
	writeTestFile(t, filepath.Join(dir, "b", "f.txt"), "needle\n")

	out := string(execute(t, grepCmd, nil, "needle", "--path", "a/f.txt"))
	if !strings.Contains(out, "a/f.txt:1:needle") {
		t.Errorf("--path answer = %q", out)
	}
	if strings.Contains(out, "b/f.txt") {
		t.Errorf("--path must search exactly one file, got:\n%s", out)
	}
}

// TestGrepPathRefusesADirectory and the flag conflicts keep --path from meaning
// two things at once.
func TestGrepPathRefusesADirectory(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a", "f.txt"), "needle\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, grepCmd, nil, "needle", "--path", "a"))
	})
}

func TestGrepPathRefusesConflictingFlags(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a", "f.txt"), "needle\n")
	for _, args := range [][]string{
		{"needle", "--path", "a/f.txt", "--dir", "a"},
		{"needle", "--path", "a/f.txt", "--include", "**/*.txt"},
	} {
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, grepCmd, nil, args...))
		})
	}
}
