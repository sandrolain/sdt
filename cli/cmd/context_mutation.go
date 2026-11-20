package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/filelock"
	"github.com/sandrolain/sdt/internal/frontmatter"
	"github.com/spf13/cobra"
	yaml "go.yaml.in/yaml/v3"
)

const (
	contextExpectedRevisionFlag = "expected-revision"
	contextDocumentSchema       = 1
)

type contextMutationConflict struct {
	Path             string
	ExpectedRevision string
	CurrentRevision  string
	Diff             string
}

func (e *contextMutationConflict) Error() string {
	return fmt.Sprintf("%s changed since it was read", e.Path)
}

func addContextExpectedRevisionFlag(cmd *cobra.Command) {
	cmd.Flags().String(contextExpectedRevisionFlag, "", "Require this full SHA-256 document revision before writing")
}

func contextExpectedRevision(cmd *cobra.Command) (string, error) {
	revision := strings.TrimSpace(getStringFlag(cmd, contextExpectedRevisionFlag, false))
	if revision == "" {
		return "", nil
	}
	if len(revision) != sha256.Size*2 {
		return "", fmt.Errorf("--%s must be a full 64-character SHA-256 hex digest", contextExpectedRevisionFlag)
	}
	if _, err := hex.DecodeString(revision); err != nil {
		return "", fmt.Errorf("--%s must be a full 64-character SHA-256 hex digest: %w", contextExpectedRevisionFlag, err)
	}
	return strings.ToLower(revision), nil
}

func contextDocumentRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func guardedContextDocWrite(path string, original []byte, proposed, expectedRevision string) error {
	proposedBytes := []byte(proposed)
	if bytes.Equal(original, proposedBytes) {
		return nil
	}
	lockPath, err := contextDocumentLockPath(path)
	if err != nil {
		return err
	}
	baseRevision := contextDocumentRevision(original)
	return filelock.WithLock(lockPath, func() error {
		current, err := os.ReadFile(path) //#nosec G304 -- resolved context document
		if err != nil {
			return err
		}
		currentRevision := contextDocumentRevision(current)
		if currentRevision != baseRevision || expectedRevision != "" && currentRevision != expectedRevision {
			expected := expectedRevision
			if expected == "" {
				expected = baseRevision
			}
			return &contextMutationConflict{
				Path:             path,
				ExpectedRevision: expected,
				CurrentRevision:  currentRevision,
				Diff: unifiedDiff(path, path, strings.Split(string(current), "\n"),
					strings.Split(proposed, "\n"), 3),
			}
		}
		if err := validateContextDocumentSchema(string(current)); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := validateContextDocumentSchema(proposed); err != nil {
			return fmt.Errorf("%s: proposed content: %w", path, err)
		}
		revision := baseRevision
		if expectedRevision != "" {
			revision = expectedRevision
		}
		return writeContextDocFileAtRevision(path, revision, proposed)
	})
}

func contextDocumentLockPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := contextProjectRoot(absolute)
	if err != nil {
		return "", err
	}
	target := absolute
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		target = resolved
	} else if !os.IsNotExist(err) {
		return "", err
	}
	lockDir := filepath.Join(root, ".sdt", "document-locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(filepath.Clean(target)))
	return filepath.Join(lockDir, hex.EncodeToString(sum[:])+".lock"), nil
}

func validateContextDocumentSchema(content string) error {
	block, err := frontmatter.Parse(content)
	if errors.Is(err, frontmatter.ErrNoFrontmatter) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot parse document schema: %w", err)
	}
	found := false
	for i := 0; i+1 < len(block.Mapping.Content); i += 2 {
		key, value := block.Mapping.Content[i], block.Mapping.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Value != "schema_version" {
			continue
		}
		if found {
			return fmt.Errorf("schema_version is duplicated")
		}
		found = true
		var version int
		if err := value.Decode(&version); err != nil {
			return fmt.Errorf("schema_version must be an integer: %w", err)
		}
		if version != contextDocumentSchema {
			return fmt.Errorf("unsupported schema_version %d (this build supports %d)", version, contextDocumentSchema)
		}
	}
	return nil
}

func exitWithContextMutationError(cmd *cobra.Command, err error) {
	var conflict *contextMutationConflict
	if errors.As(err, &conflict) {
		writeStderr(cmd, fmt.Sprintf("sdt-conflict path=%s expected_revision=%s current_revision=%s\n",
			conflict.Path, conflict.ExpectedRevision, conflict.CurrentRevision))
		if conflict.Diff != "" {
			writeStderr(cmd, "proposed diff:\n"+conflict.Diff)
		}
		exitWithDecision(cmd, "document revision changed", []string{
			"reread: sdt read -p " + conflict.Path,
			"reapply after review with --expected-revision " + conflict.CurrentRevision,
		})
		return
	}
	exitWithError(cmd, err)
}
