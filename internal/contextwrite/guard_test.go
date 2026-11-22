package contextwrite

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCreateWritesOnlyNewRootedRelativeFiles(t *testing.T) {
	root := t.TempDir()
	if err := Create(root, "context/invalid-mode", []byte("x"), 0o1000); err == nil {
		t.Fatal("Create accepted mode bits outside the permission mask")
	}
	if err := Create(root, "context/learning/profiles/a/1.yaml", []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "context", "learning", "profiles", "a", "1.yaml")
	if data, err := os.ReadFile(path); err != nil || string(data) != "version: 1\n" {
		t.Fatalf("created file = %q, %v", data, err)
	}
	if err := Create(root, "context/learning/profiles/a/1.yaml", []byte("version: 2\n"), 0o644); err == nil {
		t.Fatal("Create overwrote an existing version")
	}
	for _, target := range []string{"../outside", "/absolute", ".."} {
		if err := Create(root, target, []byte("x"), 0o644); err == nil {
			t.Errorf("Create accepted unsafe path %q", target)
		}
	}
	if err := Create(filepath.Join(root, "missing-root"), "context/file", []byte("x"), 0o600); err == nil {
		t.Fatal("Create accepted a missing project root")
	}
}

func TestCreateRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context", "learning"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "context", "learning", "profiles")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := Create(root, "context/learning/profiles/a/1.yaml", []byte("private\n"), 0o644); err == nil {
		t.Fatal("Create wrote through a symlink outside the project")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("outside directory entries = %v, %v", entries, err)
	}
}

func TestWriteReplacesOnlyExpectedRevision(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "wiki", "page.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := []byte("old\n")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, path, Digest(old), []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("document = %q, want new content", got)
	}
}

func TestConcurrentWritersAcceptOnlyOneSnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "notes", "shared.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := []byte("shared snapshot\n")
	if err := os.WriteFile(path, snapshot, 0o644); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, proposal := range []string{"writer one\n", "writer two\n"} {
		go func(content string) {
			ready.Done()
			<-start
			results <- Write(root, path, Digest(snapshot), []byte(content))
		}(proposal)
	}
	ready.Wait()
	close(start)
	first, second := <-results, <-results
	successes, conflicts := 0, 0
	for _, err := range []error{first, second} {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent write error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d; want one each", successes, conflicts)
	}
}

func TestWriteRefusesReadOnlyDocumentWithoutReplacingIt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "notes", "readonly.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("read only\n")
	if err := os.WriteFile(path, original, 0o444); err != nil {
		t.Fatal(err)
	}
	err := Write(root, path, Digest(original), []byte("replacement\n"))
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Write error = %v, want permission error", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("document = %q after refused write, want original", got)
	}
}

func TestWriteRefusesStaleRevisionAndPreservesDocument(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "wiki", "page.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Write(root, path, Digest([]byte("stale\n")), []byte("proposal\n"))
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Write error = %v, want ErrConflict", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "current\n" {
		t.Fatalf("document = %q after conflict, want current content", got)
	}
}

func TestWriteRejectsOutsideRootAndMissingTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, outside, Digest([]byte("x")), []byte("y")); err == nil {
		t.Fatal("outside write accepted")
	}
	missing := filepath.Join(root, "context", "missing.md")
	if err := Write(root, missing, Digest(nil), []byte("y")); err == nil {
		t.Fatal("missing target write accepted")
	}
}

func TestMoveValidatesAndRenames(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "context", "old.md")
	dst := filepath.Join(root, "context", "nested", "new.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("document\n")
	if err := os.WriteFile(src, content, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Move(root, src, dst, Digest([]byte("stale"))); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Move error = %v", err)
	}
	if err := Move(root, src, filepath.Join(t.TempDir(), "outside"), Digest(content)); err == nil {
		t.Fatal("outside move accepted")
	}
	if err := Move(root, src, dst, Digest(content)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("destination = %q", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source remains: %v", err)
	}
	if err := Move(root, dst, src, Digest(content)); err != nil {
		t.Fatal(err)
	}
	if err := Move(root, dst, src, Digest(content)); err == nil {
		t.Fatal("missing source move accepted")
	}
}

func TestMoveRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "context", "a.md"), filepath.Join(root, "context", "b.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("a")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Move(root, src, dst, Digest(data)); err == nil {
		t.Fatal("existing destination accepted")
	}
}

func TestMoveAcrossDevicesCopiesAndRemovesSource(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "nested", "dst")
	data := []byte("copy fallback")
	if err := os.WriteFile(src, data, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := moveAcrossDevices(src, dst, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("copied data = %q", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source remains: %v", err)
	}
	if err := moveAcrossDevices(src, filepath.Join(root, "missing", "dst"), data); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := moveAcrossDevices(src, dst, data); err == nil {
		t.Fatal("existing destination accepted")
	}
}

func TestConflictErrorFormatsRevisionDetails(t *testing.T) {
	err := (&ConflictError{Expected: "expected", Actual: "actual"}).Error()
	if err == "" {
		t.Fatal("empty conflict detail")
	}
}
