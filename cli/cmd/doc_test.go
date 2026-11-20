package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeMarkdownTree(t *testing.T) {
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "generated.md")
	textPath := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(markdownPath, []byte("line with space \t\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(textPath, []byte("line with space \t\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := normalizeMarkdownTree(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line with space\n" {
		t.Fatalf("normalized markdown = %q", got)
	}
	got, err = os.ReadFile(textPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line with space \t\n\n" {
		t.Fatalf("non-markdown file changed: %q", got)
	}
}

func TestNormalizeMarkdownTreeRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	externalPath := filepath.Join(outside, "external.md")
	if err := os.WriteFile(externalPath, []byte("leave this unchanged \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "external.md")
	if err := os.Symlink(externalPath, linkPath); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	if err := normalizeMarkdownTree(root); err == nil {
		t.Fatal("normalizeMarkdownTree succeeded through a symlink outside its root")
	}
	got, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "leave this unchanged \n" {
		t.Fatalf("outside file changed through symlink: %q", got)
	}
}
