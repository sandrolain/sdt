package contextwrite

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

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
