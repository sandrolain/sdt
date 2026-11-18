package delegation

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadArtifact confines reads, including symlinks, to the private delegation root.
func (s *Store) ReadArtifact(id, name string) ([]byte, error) {
	if !safeID.MatchString(id) || !filepath.IsLocal(name) || filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid task artifact")
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // Preserve the read error.
	return root.ReadFile(filepath.Join("tasks", id, "artifacts", name))
}
