//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCommandPrintsLineRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, fileReadCmd, nil, "-p", path, "--lines", "2:3")
	if got := string(out); got != "two\nthree\n" {
		t.Errorf("range read = %q, want %q", got, "two\nthree\n")
	}
	out = execute(t, fileReadCmd, nil, "-p", path)
	if got := string(out); got != "one\ntwo\nthree\nfour\n" {
		t.Errorf("full read = %q", got)
	}
}

func TestReadCommandClampsUpperBoundAndErrorsPastTheEnd(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, fileReadCmd, nil, "-p", path, "--lines", "1:9999")
	if got := string(out); got != "one\ntwo\n" {
		t.Errorf("clamped read = %q, want the whole file", got)
	}
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileReadCmd, nil, "-p", path, "--lines", "99:120")
		return ""
	})
}

func TestReadCommandRejectsInvalidRanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{"2:1", "0:2", "a:b", "2"} {
		shouldExitWithCode(t, 1, func() string {
			execute(t, fileReadCmd, nil, "-p", path, "--lines", spec)
			return ""
		})
	}
}

func TestShowCommandSharesTheLineRangeContract(t *testing.T) {
	t.Chdir(t.TempDir())
	doc := "context/notes/sample.md"
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("---\nkind: notes\n---\n\nalpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextShowCmd, nil, doc, "--lines", "5:6")
	if !strings.Contains(string(out), "alpha") || !strings.Contains(string(out), "beta") {
		t.Errorf("range read = %q", out)
	}
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextShowCmd, nil, doc, "--lines", "99:120")
		return ""
	})
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextShowCmd, nil, doc, "--lines", "6:5")
		return ""
	})
}

func TestReadCommandHelpCarriesTheDeclaredShape(t *testing.T) {
	for _, part := range []string{"WHEN —", "WHEN NOT —", "RETURNS —", "EXAMPLE —"} {
		if !strings.Contains(fileReadCmd.Long, part) {
			t.Errorf("read help is missing the %s part", part)
		}
	}
}
