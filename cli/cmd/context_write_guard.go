package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwrite"
)

func writeContextDocFile(path string, expected []byte, content string) error {
	root, err := contextProjectRoot(path)
	if err != nil {
		return err
	}
	return contextwrite.Write(root, path, contextwrite.Digest(expected), []byte(content))
}

func writeContextDocFileAtRevision(path, revision, content string) error {
	root, err := contextProjectRoot(path)
	if err != nil {
		return err
	}
	return contextwrite.Write(root, path, revision, []byte(content))
}

func createContextDocFile(path, content string) error {
	return createContextDocFileMode(path, []byte(content), 0o644)
}

func createContextDocFileMode(path string, content []byte, mode os.FileMode) error {
	if _, err := contextProjectRoot(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //#nosec G304 -- path is constrained to project context above
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()       //nolint:errcheck // Best-effort cleanup while returning the write error.
		_ = os.Remove(path) //nolint:errcheck // Best-effort cleanup of our exclusive file.
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()       //nolint:errcheck // Best-effort cleanup while returning the sync error.
		_ = os.Remove(path) //nolint:errcheck // Best-effort cleanup of our exclusive file.
		return err
	}
	return f.Close()
}

func contextProjectRoot(path string) (string, error) {
	if root := sdtProjectRoot(); root != "" {
		rootAbs, rootErr := filepath.Abs(root)
		targetAbs, targetErr := filepath.Abs(path)
		if rootErr == nil && targetErr == nil {
			if rel, relErr := filepath.Rel(rootAbs, targetAbs); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return rootAbs, nil
			}
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == sdtWorkDir {
			return filepath.Dir(dir), nil
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "", errors.New("cannot locate context root for guarded write")
}

func validateExpectedRevision(revision string) error {
	if len(revision) != 64 {
		return fmt.Errorf("--expect-sha256 must be a full 64-character SHA-256 digest")
	}
	if _, err := hex.DecodeString(revision); err != nil || strings.ToLower(revision) != revision {
		return fmt.Errorf("--expect-sha256 must be lowercase hexadecimal")
	}
	return nil
}
