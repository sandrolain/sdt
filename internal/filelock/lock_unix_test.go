//go:build unix

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWithLockRunsCallbackAndReleasesAfterError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.lock")
	wantErr := errors.New("callback failed")
	called := false

	if err := WithLock(path, func() error {
		called = true
		return wantErr
	}); err != wantErr {
		t.Fatalf("WithLock error = %v, want %v", err, wantErr)
	}
	if !called {
		t.Fatal("callback was not called")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %#o, want 0600", got)
	}

	called = false
	if err := WithLock(path, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("second WithLock call: %v", err)
	}
	if !called {
		t.Fatal("second callback was not called after the first error")
	}
}
