// Package contextwrite protects managed context documents from concurrent
// writes by participating SDT commands.
package contextwrite

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/gofrs/flock"
)

// ErrConflict means the document changed after the caller read it.
var ErrConflict = errors.New("context document changed since it was read")

// ConflictError reports the expected and observed full-file revisions.
type ConflictError struct {
	Expected string
	Actual   string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s (expected sha256 %s, found %s); reread the document and retry", ErrConflict, e.Expected, e.Actual)
}

func (e *ConflictError) Unwrap() error { return ErrConflict }

// Digest returns the lowercase full SHA-256 of the exact document bytes.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Write replaces path only if its current bytes still match expected. A
// project-local lock serializes participating writers; the temporary file is
// created beside the target so rename stays on the same filesystem.
func Write(projectRoot, path, expectedRevision string, proposed []byte) error {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("context write target %q is outside project root %q", path, projectRoot)
	}
	unlock, err := lockPaths(root, target)
	if err != nil {
		return err
	}
	defer unlock()

	current, err := os.ReadFile(target) //#nosec G304 -- target is constrained to the project root above
	if err != nil {
		return fmt.Errorf("read context document before write: %w", err)
	}
	want, got := expectedRevision, Digest(current)
	if want != got {
		return &ConflictError{Expected: want, Actual: got}
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat context document before write: %w", err)
	}
	if info.Mode().Perm()&0o222 == 0 {
		return fmt.Errorf("replace context document: %w", os.ErrPermission)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".sdt-write-*")
	if err != nil {
		return fmt.Errorf("create context write temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() //nolint:errcheck // Best-effort cleanup after rename or failure.
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close() //nolint:errcheck // Best-effort cleanup while returning the chmod error.
		return fmt.Errorf("set context write mode: %w", err)
	}
	if _, err := tmp.Write(proposed); err != nil {
		_ = tmp.Close() //nolint:errcheck // Best-effort cleanup while returning the write error.
		return fmt.Errorf("write context temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // Best-effort cleanup while returning the sync error.
		return fmt.Errorf("sync context temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close context temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("replace context document: %w", err)
	}
	return nil
}

// Move renames a document only if its current bytes still match the caller's
// revision. Both path locks are acquired in stable order to avoid deadlocks.
func Move(projectRoot, source, destination, expectedRevision string) error {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	src, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	for _, target := range []string{src, dst} {
		rel, relErr := filepath.Rel(root, target)
		if relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("context move target %q is outside project root %q", target, root)
		}
	}
	unlock, err := lockPaths(root, src, dst)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := os.ReadFile(src) //#nosec G304 -- source is constrained to the project root above
	if err != nil {
		return fmt.Errorf("read context document before move: %w", err)
	}
	got := Digest(current)
	if got != expectedRevision {
		return &ConflictError{Expected: expectedRevision, Actual: got}
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("context move destination already exists: %s", dst)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("create context move destination directory: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		if !errors.Is(err, syscall.EXDEV) {
			return fmt.Errorf("move context document: %w", err)
		}
		info, statErr := os.Stat(src)
		if statErr != nil {
			return statErr
		}
		out, createErr := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()) //#nosec G304 -- destination is constrained to the project root above
		if createErr != nil {
			return createErr
		}
		if _, writeErr := out.Write(current); writeErr != nil {
			_ = out.Close()    //nolint:errcheck // Best-effort cleanup while returning the write error.
			_ = os.Remove(dst) //nolint:errcheck // Best-effort cleanup of our exclusive destination.
			return writeErr
		}
		if syncErr := out.Sync(); syncErr != nil {
			_ = out.Close()    //nolint:errcheck // Best-effort cleanup while returning the sync error.
			_ = os.Remove(dst) //nolint:errcheck // Best-effort cleanup of our exclusive destination.
			return syncErr
		}
		if closeErr := out.Close(); closeErr != nil {
			_ = os.Remove(dst) //nolint:errcheck // Best-effort cleanup of our exclusive destination.
			return closeErr
		}
		if removeErr := os.Remove(src); removeErr != nil {
			_ = os.Remove(dst) //nolint:errcheck // Best-effort cleanup of our exclusive destination.
			return removeErr
		}
	}
	return nil
}

func lockPaths(root string, targets ...string) (func(), error) {
	lockDir := filepath.Join(root, ".sdt", "locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create context lock directory: %w", err)
	}
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return nil, err
		}
		paths = append(paths, filepath.Join(lockDir, Digest([]byte(filepath.ToSlash(rel)))+".lock"))
	}
	sort.Strings(paths)
	locks := make([]*flock.Flock, 0, len(paths))
	for _, path := range paths {
		lock := flock.New(path, flock.SetPermissions(0o600))
		if err := lock.Lock(); err != nil {
			for i := len(locks) - 1; i >= 0; i-- {
				_ = locks[i].Unlock() //nolint:errcheck // Best-effort cleanup while returning the lock error.
			}
			return nil, fmt.Errorf("lock context document: %w", err)
		}
		locks = append(locks, lock)
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			_ = locks[i].Unlock() //nolint:errcheck // Best-effort unlock after the protected operation.
		}
	}, nil
}
