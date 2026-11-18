package delegation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadArtifactConfinesPathsAndSymlinks(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "task-1"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(store.Root(), "tasks", "task-1", "artifacts")
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte("brief"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadArtifact("task-1", "brief.md")
	if err != nil || string(got) != "brief" {
		t.Fatalf("%q, %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.md")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range [][2]string{{"task-1", "escape.md"}, {"../outside", "brief.md"}, {"task-1", "../brief.md"}, {"task-1", outside}, {"task-1", "."}} {
		if _, err := store.ReadArtifact(tc[0], tc[1]); err == nil {
			t.Fatalf("accepted %v", tc)
		}
	}
}
