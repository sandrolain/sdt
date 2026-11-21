package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCopiesExactSourceBytes(t *testing.T) {
	dir := runInIsolatedTempDir(t)
	source := filepath.Join(dir, "source.yaml")
	target := filepath.Join(dir, "target.yaml")
	want := []byte("metadata:\r\n  retained: true\n")
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}
	execute(t, fileWriteCmd, nil, "-p", "target.yaml", "--source", source)
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("copied bytes differ: got %q, want %q", got, want)
	}
}
