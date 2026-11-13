package figure

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// convertedDir is where `doc2md --keep` stores the converted corpus.
const convertedDir = "context/refs/converted"

// DisplayPath renders a path the way the manifest records it: relative to the
// project root when the file lives under it, as typed otherwise.
func DisplayPath(root, p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// FindConverted locates the converted Markdown of an EPUB under the project's
// kept corpus. The canonical name is <source-id>-<sha256[:12]>.md, the name
// `doc2md --keep` gives it; a slug variant (underscores folded to dashes) is
// accepted as a fallback, and several candidates are an error that names them.
func FindConverted(root, sourceID, epubPath string) (display, read string, err error) {
	f, errOpen := os.Open(epubPath) //#nosec G304 -- caller-provided input path
	if errOpen != nil {
		return "", "", fmt.Errorf("unreadable input: %s: %w", epubPath, errOpen)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only handle
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", "", fmt.Errorf("cannot digest %s: %w", epubPath, err)
	}
	sha12 := hex.EncodeToString(h.Sum(nil))[:12]

	dir := filepath.Join(root, filepath.FromSlash(convertedDir))
	canonical := filepath.Join(dir, fmt.Sprintf("%s-%s.md", sourceID, sha12))
	if _, statErr := os.Stat(canonical); statErr == nil {
		return DisplayPath(root, canonical), canonical, nil
	}

	slug := strings.ReplaceAll(sourceID, "_", "-")
	patterns := []string{sourceID, slug}
	var matches []string
	seen := map[string]bool{}
	for _, stem := range patterns {
		found, globErr := filepath.Glob(filepath.Join(dir, stem+"-*.md"))
		if globErr != nil {
			continue
		}
		for _, m := range found {
			abs, absErr := filepath.Abs(m)
			if absErr != nil {
				continue
			}
			if !seen[abs] {
				seen[abs] = true
				matches = append(matches, m)
			}
		}
	}
	switch len(matches) {
	case 1:
		return DisplayPath(root, matches[0]), matches[0], nil
	case 0:
		return "", "", fmt.Errorf("no converted Markdown for %s under %s: run `sdt doc2md %s --keep` first", sourceID, convertedDir, epubPath)
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, filepath.Base(m))
		}
		return "", "", fmt.Errorf("ambiguous converted Markdown for %s: %s", sourceID, strings.Join(names, ", "))
	}
}
