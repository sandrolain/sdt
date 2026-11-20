// Package metacomp reads and validates versioned metadata companions.
package metacomp

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const SchemaVersion = "1.0"
const CompanionSuffix = ".meta.yaml"

type Artifact struct {
	Role   string `yaml:"role" json:"role"`
	Path   string `yaml:"path" json:"path"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

// Document is represented as a map so unknown extension keys survive reads,
// migrations and updates. Known identity fields are validated separately.
type Document map[string]any

func CompanionPath(targetPath string) string { return targetPath + CompanionSuffix }

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func New(targetPath, targetDigest string) Document {
	return Document{
		"schema_version": SchemaVersion,
		"target":         map[string]any{"path": filepath.ToSlash(targetPath), "sha256": targetDigest},
	}
}

func Decode(data []byte) (Document, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("metadata companion must be a YAML mapping")
	}
	return Document(doc), nil
}

func Encode(doc Document) ([]byte, error) { return yaml.Marshal(map[string]any(doc)) }

// ReadTarget loads the complete-filename companion for a context-relative
// artifact and verifies both the path association and exact target bytes.
func ReadTarget(contextRoot, targetPath string) (Document, bool, error) {
	targetPath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(targetPath)))
	if !strings.HasPrefix(targetPath, "context/") || filepath.IsAbs(targetPath) || strings.HasPrefix(targetPath, "../") {
		return nil, false, fmt.Errorf("target path %q must be context-relative", targetPath)
	}
	rootAbs, err := filepath.Abs(contextRoot)
	if err != nil {
		return nil, false, err
	}
	companion := filepath.Join(rootAbs, filepath.FromSlash(targetPath+CompanionSuffix))
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, false, err
	}
	resolvedCompanion, err := filepath.EvalSymlinks(companion)
	if err == nil {
		inside, relErr := filepath.Rel(resolvedRoot, resolvedCompanion)
		if relErr != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			return nil, false, fmt.Errorf("companion path %q escapes project root", targetPath+CompanionSuffix)
		}
	} else if !os.IsNotExist(err) {
		return nil, false, err
	}
	data, err := os.ReadFile(companion) // #nosec G304 -- paths are checked below and are corpus-relative
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	doc, err := Decode(data)
	if err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", filepath.ToSlash(companion), err)
	}
	if err := ValidateTarget(contextRoot, targetPath, doc); err != nil {
		return nil, true, fmt.Errorf("validate %s: %w", filepath.ToSlash(companion), err)
	}
	return doc, true, nil
}

func ValidateTarget(contextRoot, targetPath string, doc Document) error {
	version, ok := doc["schema_version"].(string)
	if !ok || version != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported (want %q)", version, SchemaVersion)
	}
	target, ok := stringMap(doc["target"])
	if !ok {
		return fmt.Errorf("target must be a mapping with path and sha256")
	}
	path, ok := target["path"].(string)
	if !ok {
		return fmt.Errorf("target.path is required")
	}
	if filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) != targetPath {
		return fmt.Errorf("target path %q does not match artifact %q", path, targetPath)
	}
	if err := verifyFile(contextRoot, path, target["sha256"]); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	if raw, exists := doc["related_artifacts"]; exists {
		items, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("related_artifacts must be a sequence")
		}
		for i, item := range items {
			m, ok := stringMap(item)
			if !ok {
				return fmt.Errorf("related_artifacts[%d] must be a mapping", i)
			}
			role, ok := m["role"].(string)
			if !ok || strings.TrimSpace(role) == "" {
				return fmt.Errorf("related_artifacts[%d].role is required", i)
			}
			path, ok := m["path"].(string)
			if !ok {
				return fmt.Errorf("related_artifacts[%d].path is required", i)
			}
			if err := verifyFile(contextRoot, path, m["sha256"]); err != nil {
				return fmt.Errorf("related_artifacts[%d]: %w", i, err)
			}
		}
	}
	return nil
}

func verifyFile(root, rel string, want any) error {
	path, ok := want.(string)
	if !ok || len(path) != 64 || strings.ToLower(path) != path {
		return fmt.Errorf("sha256 must be a lowercase full SHA-256 digest")
	}
	if _, err := hex.DecodeString(path); err != nil {
		return fmt.Errorf("sha256 must be a lowercase full SHA-256 digest")
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q must be context-relative", rel)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target := filepath.Join(rootAbs, rel)
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return err
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	if inside, err := filepath.Rel(resolvedRoot, resolvedTarget); err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes context root", rel)
	}
	rootFS, err := os.OpenRoot(rootAbs)
	if err != nil {
		return err
	}
	data, err := rootFS.ReadFile(rel)
	closeErr := rootFS.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	got := Digest(data)
	if got != path {
		return fmt.Errorf("sha256 mismatch for %q (expected %s, found %s)", filepath.ToSlash(rel), path, got)
	}
	return nil
}

func stringMap(value any) (map[string]any, bool) {
	switch m := value.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for key, value := range m {
			text, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[text] = value
		}
		return out, true
	default:
		return nil, false
	}
}
