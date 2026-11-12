package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteAppendCreatesAndKeeps proves the cat >> shape: a missing file is
// created, an existing one keeps its prior bytes and its mode.
func TestWriteAppendCreatesAndKeeps(t *testing.T) {
	dir := runInTempDir(t)
	fresh := filepath.Join(dir, "fresh.txt")
	execute(t, fileWriteCmd, []byte("one\n"), "-p", "fresh.txt", "--append")
	got, err := os.ReadFile(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\n" {
		t.Errorf("created file = %q", got)
	}
	info, err := os.Stat(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("created mode = %v, want 0644", info.Mode().Perm())
	}

	kept := filepath.Join(dir, "kept.txt")
	writeTestFile(t, kept, "head\n")
	if err := os.Chmod(kept, 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, fileWriteCmd, []byte("tail\n"), "-p", "kept.txt", "--append")
	got, err = os.ReadFile(kept)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "head\ntail\n" {
		t.Errorf("appended file = %q", got)
	}
	info, err = os.Stat(kept)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %v, want 0750 preserved", info.Mode().Perm())
	}
}

// TestWriteAppendRefusesDirectoryCorpusAndConflict covers the guards: a
// directory target, a corpus document, and the --overwrite conflict.
func TestWriteAppendRefusesDirectoryCorpusAndConflict(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileWriteCmd, []byte("x\n"), "-p", ".", "--append")
		return ""
	})
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/x.md", "---\nkind: notes\nsummary: s\n---\nbody\n")
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileWriteCmd, []byte("x\n"), "-p", "context/notes/x.md", "--append")
		return ""
	})
	writeTestFile(t, "f.txt", "a\n")
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileWriteCmd, []byte("x\n"), "-p", "f.txt", "--append", "--overwrite")
		return ""
	})
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileWriteCmd, []byte("x\n"), "-p", "f.txt", "--append", "--multi")
		return ""
	})
}
