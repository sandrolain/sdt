package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// editFixture writes a file in a temporary working directory and returns its
// absolute path and name.
func editFixture(t *testing.T, name, content string) (abs, rel string) {
	t.Helper()
	dir := runInTempDir(t)
	abs = filepath.Join(dir, name)
	writeTestFile(t, abs, content)
	return abs, name
}

// TestEditReplacesOneLiteral is the happy path: the diff is printed and the
// file carries the replacement.
func TestEditReplacesOneLiteral(t *testing.T) {
	abs, rel := editFixture(t, "f.go", "alpha\nbeta\ngamma\n")
	out := string(execute(t, editCmd, nil, "-p", rel, "--old", "beta", "--new", "BETA"))
	if !strings.Contains(out, "-beta") || !strings.Contains(out, "+BETA") {
		t.Errorf("edit must print the diff, got:\n%s", out)
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alpha\nBETA\ngamma\n" {
		t.Errorf("file = %q", got)
	}
}

// TestEditRefusesZeroAndMany proves the guard: the two cases a guess would
// corrupt a source file are refusals, not best efforts.
func TestEditRefusesZeroAndMany(t *testing.T) {
	_, rel := editFixture(t, "f.txt", "x\nx\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "nope", "--new", "y"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "x", "--new", "y"))
	})
}

// TestEditAllReplacesEveryOccurrence proves --all is the explicit override the
// refusal points at.
func TestEditAllReplacesEveryOccurrence(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "x\nx\nx\n")
	execute(t, editCmd, nil, "-p", rel, "--old", "x", "--new", "y", "--all")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "y\ny\ny\n" {
		t.Errorf("file = %q", got)
	}
}

// TestEditDryRunWritesNothing proves the review step: the diff is printed and
// the file is byte-identical afterwards.
func TestEditDryRunWritesNothing(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "one\n")
	out := string(execute(t, editCmd, nil, "-p", rel, "--old", "one", "--new", "two", "--dry-run"))
	if !strings.Contains(out, "+two") {
		t.Errorf("dry run must print the diff, got:\n%s", out)
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\n" {
		t.Errorf("dry run wrote the file: %q", got)
	}
}

// TestEditEmptyReplacementDeletes proves --new is required but may be empty:
// deleting a literal is a real edit, an absent --new is a caller bug.
func TestEditEmptyReplacementDeletes(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "keep\ndrop\n")
	execute(t, editCmd, nil, "-p", rel, "--old", "drop\n", "--new", "")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep\n" {
		t.Errorf("file = %q", got)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "keep"))
	})
}

// TestEditRefusesAnEmptyLiteral and the two-source case: both would make the
// verb's answer depend on a precedence rule nobody can inspect.
func TestEditRefusesAnEmptyLiteral(t *testing.T) {
	_, rel := editFixture(t, "f.txt", "x\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "", "--new", "y"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", rel, "--old", "x", "--old-file", "x", "--new", "y"))
	})
}

// TestEditCRLFStillMatches proves a shell literal with \n matches a CRLF file,
// and the file keeps its line endings.
func TestEditCRLFStillMatches(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "alpha\r\nbeta\r\n")
	execute(t, editCmd, nil, "-p", rel, "--old", "alpha\nbeta", "--new", "one\ntwo")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\r\ntwo\r\n" {
		t.Errorf("file = %q, want CRLF preserved", got)
	}
}

// TestEditPreservesTheMode proves a source file does not lose its permissions,
// the defect `sdt write` has: a verb meant for code must not write 0600.
func TestEditPreservesTheMode(t *testing.T) {
	abs, rel := editFixture(t, "f.sh", "#!/bin/sh\necho one\n")
	if err := os.Chmod(abs, 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, editCmd, nil, "-p", rel, "--old", "one", "--new", "two")
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %v, want 0750", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(abs))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".sdt-edit-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestEditRefusesBinarySymlinkAndCorpus covers the three "must not touch"
// cases: a binary file, a symlink, and a corpus document.
func TestEditRefusesBinarySymlinkAndCorpus(t *testing.T) {
	abs, rel := editFixture(t, "f.txt", "x\n")
	writeTestFile(t, filepath.Join(filepath.Dir(abs), "b.bin"), "a\x00b")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", "b.bin", "--old", "a", "--new", "c"))
	})
	link := filepath.Join(filepath.Dir(abs), "link.txt")
	if err := os.Symlink(abs, link); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", "link.txt", "--old", "x", "--new", "y"))
	})
	_ = rel
}

// TestEditRefusesTheCorpusAndOutsideTheRoot proves the two scopes: the corpus
// keeps one mutator, and the project bounds what the verb can touch.
func TestEditRefusesTheCorpusAndOutsideTheRoot(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/x.md", "---\nkind: notes\nsummary: s\n---\nbody\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", "context/notes/x.md", "--old", "body", "--new", "b"))
	})
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")
	writeTestFile(t, outside, "x\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "-p", outside, "--old", "x", "--new", "y"))
	})
}

// TestEditJSONAnswer proves the machine-readable result matches the mutators
// already shipped.
func TestEditJSONAnswer(t *testing.T) {
	_, rel := editFixture(t, "f.txt", "x\n")
	out := execute(t, editCmd, nil, "-p", rel, "--old", "x", "--new", "y", "--format", "json")
	var payload struct {
		Path        string `json:"path"`
		Changed     bool   `json:"changed"`
		Occurrences int    `json:"occurrences"`
		Replaces    int    `json:"replaces"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json answer does not parse: %v\n%s", err, out)
	}
	if payload.Path != rel || !payload.Changed || payload.Occurrences != 1 || payload.Replaces != 1 || payload.DryRun {
		t.Errorf("json answer = %+v", payload)
	}
}
