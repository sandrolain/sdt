// Package materialpackage writes and validates portable learning-material ZIPs.
package materialpackage

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ManifestVersion = "1.1"

const ManifestPath = "manifest.json"
const InstructionsPath = "README.md"

const (
	PrivacyNone     = "none"
	PrivacyPersonal = "personal"
	PrivacyReal     = "real"
	PrivacyCleared  = "cleared"
)

// Schema is the versioned JSON Schema consumers can use independently.
//
//go:embed schema.json
var Schema []byte

type Manifest struct {
	SchemaVersion string      `json:"schema_version"`
	GeneratedAt   string      `json:"generated_at"`
	Target        string      `json:"target"`
	Artifacts     []Artifact  `json:"artifacts"`
	Exclusions    []Exclusion `json:"exclusions"`
}

type Artifact struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Path             string            `json:"path"`
	SHA256           string            `json:"sha256"`
	SourceVersions   map[string]string `json:"source_versions,omitempty"`
	Privacy          string            `json:"privacy"`
	PrivacyClearance *PrivacyClearance `json:"privacy_clearance,omitempty"`
}

// PrivacyClearance records the review basis for an artifact containing personal
// or real-world data. A missing clearance must never be interpreted as approval.
type PrivacyClearance struct {
	Status string `json:"status"`
	Basis  string `json:"basis"`
}

type Exclusion struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type InputArtifact struct {
	ID               string
	Type             string
	Path             string
	Data             []byte
	SourceVersions   map[string]string
	Privacy          string
	PrivacyClearance *PrivacyClearance
}

// Build writes a ZIP with a versioned manifest and self-contained consumer
// instructions. Input paths are package-relative and are preserved as supplied.
func Build(target string, artifacts []InputArtifact, exclusions []Exclusion, instructions string, generatedAt time.Time) ([]byte, error) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339),
		Target:        strings.TrimSpace(target),
		Artifacts:     make([]Artifact, 0, len(artifacts)),
		Exclusions:    append([]Exclusion(nil), exclusions...),
	}
	files := make(map[string][]byte, len(artifacts)+2)
	for _, item := range artifacts {
		if err := validateRelativePath(item.Path); err != nil {
			return nil, fmt.Errorf("artifact %q: %w", item.ID, err)
		}
		if item.Path == ManifestPath || item.Path == InstructionsPath {
			return nil, fmt.Errorf("artifact %q uses reserved package path %q", item.ID, item.Path)
		}
		if _, exists := files[item.Path]; exists {
			return nil, fmt.Errorf("duplicate package path %q", item.Path)
		}
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Type) == "" {
			return nil, fmt.Errorf("artifact id and type are required for %q", item.Path)
		}
		digest := sha256.Sum256(item.Data)
		manifest.Artifacts = append(manifest.Artifacts, Artifact{
			ID:               item.ID,
			Type:             item.Type,
			Path:             item.Path,
			SHA256:           hex.EncodeToString(digest[:]),
			SourceVersions:   cloneStringMap(item.SourceVersions),
			Privacy:          item.Privacy,
			PrivacyClearance: clonePrivacyClearance(item.PrivacyClearance),
		})
		files[item.Path] = item.Data
	}
	if manifest.Target == "" {
		return nil, errors.New("package target is required")
	}
	if len(manifest.Artifacts) == 0 {
		return nil, errors.New("package must contain at least one artifact")
	}
	if strings.TrimSpace(instructions) == "" {
		return nil, errors.New("consumer instructions are required")
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool { return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path })
	sort.Slice(manifest.Exclusions, func(i, j int) bool { return manifest.Exclusions[i].ID < manifest.Exclusions[j].ID })
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	if err := validateMarkdownLinks(files, manifest); err != nil {
		return nil, err
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode package manifest: %w", err)
	}
	files[ManifestPath] = append(manifestBytes, '\n')
	files[InstructionsPath] = []byte(instructions)

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	paths := make([]string, 0, len(files))
	for filePath := range files {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	for _, filePath := range paths {
		header := &zip.FileHeader{Name: filePath, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create package entry %q: %w", filePath, err)
		}
		if _, err := entry.Write(files[filePath]); err != nil {
			return nil, fmt.Errorf("write package entry %q: %w", filePath, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close package ZIP: %w", err)
	}
	return archive.Bytes(), nil
}

func clonePrivacyClearance(value *PrivacyClearance) *PrivacyClearance {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// Validate checks the package manifest, every referenced digest, and all ZIP
// entry paths without extracting content to disk.
func Validate(data []byte) (Manifest, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, fmt.Errorf("open package ZIP: %w", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, entry := range reader.File {
		if err := validateRelativePath(entry.Name); err != nil {
			return Manifest{}, fmt.Errorf("ZIP entry: %w", err)
		}
		if _, exists := files[entry.Name]; exists {
			return Manifest{}, fmt.Errorf("duplicate ZIP entry %q", entry.Name)
		}
		stream, err := entry.Open()
		if err != nil {
			return Manifest{}, fmt.Errorf("open ZIP entry %q: %w", entry.Name, err)
		}
		contents, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return Manifest{}, fmt.Errorf("read ZIP entry %q: %w", entry.Name, readErr)
		}
		if closeErr != nil {
			return Manifest{}, fmt.Errorf("close ZIP entry %q: %w", entry.Name, closeErr)
		}
		files[entry.Name] = contents
	}
	manifestData, ok := files[ManifestPath]
	if !ok {
		return Manifest{}, fmt.Errorf("package is missing %s", ManifestPath)
	}
	if _, ok := files[InstructionsPath]; !ok {
		return Manifest{}, fmt.Errorf("package is missing %s", InstructionsPath)
	}
	manifest, err := DecodeManifest(manifestData)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	for _, artifact := range manifest.Artifacts {
		contents, ok := files[artifact.Path]
		if !ok {
			return Manifest{}, fmt.Errorf("manifest artifact %q is missing from ZIP", artifact.Path)
		}
		digest := sha256.Sum256(contents)
		if hex.EncodeToString(digest[:]) != artifact.SHA256 {
			return Manifest{}, fmt.Errorf("artifact %q digest mismatch", artifact.Path)
		}
	}
	if err := validateMarkdownLinks(files, manifest); err != nil {
		return Manifest{}, err
	}
	allowed := map[string]bool{ManifestPath: true, InstructionsPath: true}
	for _, artifact := range manifest.Artifacts {
		allowed[artifact.Path] = true
	}
	for filePath := range files {
		if !allowed[filePath] {
			return Manifest{}, fmt.Errorf("ZIP contains unlisted entry %q", filePath)
		}
	}
	return manifest, nil
}

var markdownLinkPattern = regexp.MustCompile(`!?(?:\[[^\]]*\])\(([^)]+)\)`)

func validateMarkdownLinks(files map[string][]byte, manifest Manifest) error {
	for _, artifact := range manifest.Artifacts {
		if !strings.HasSuffix(strings.ToLower(artifact.Path), ".md") {
			continue
		}
		for _, match := range markdownLinkPattern.FindAllSubmatch(files[artifact.Path], -1) {
			target := strings.TrimSpace(string(match[1]))
			if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "//") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "data:") {
				continue
			}
			linkPath := strings.SplitN(strings.SplitN(target, "?", 2)[0], "#", 2)[0]
			if linkPath == "" {
				continue
			}
			if strings.Contains(linkPath, "\\") || strings.HasPrefix(linkPath, "/") {
				return fmt.Errorf("markdown artifact %q has a non-portable link %q", artifact.Path, target)
			}
			resolved := path.Clean(path.Join(path.Dir(artifact.Path), linkPath))
			if resolved == ".." || strings.HasPrefix(resolved, "../") {
				return fmt.Errorf("markdown artifact %q has a link outside the package: %q", artifact.Path, target)
			}
			if _, ok := files[resolved]; !ok {
				return fmt.Errorf("markdown artifact %q links to missing package file %q", artifact.Path, target)
			}
		}
	}
	return nil
}

func DecodeManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode package manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, errors.New("decode package manifest: trailing JSON value")
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestVersion {
		return fmt.Errorf("manifest schema_version %q is unsupported (want %q)", m.SchemaVersion, ManifestVersion)
	}
	if _, err := time.Parse(time.RFC3339, m.GeneratedAt); err != nil {
		return fmt.Errorf("manifest generated_at must be RFC3339: %w", err)
	}
	if strings.TrimSpace(m.Target) == "" {
		return errors.New("manifest target is required")
	}
	if len(m.Artifacts) == 0 {
		return errors.New("manifest must contain at least one artifact")
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	for i, artifact := range m.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" || strings.TrimSpace(artifact.Type) == "" {
			return fmt.Errorf("manifest artifact %d requires id and type", i+1)
		}
		if err := validateRelativePath(artifact.Path); err != nil {
			return fmt.Errorf("manifest artifact %q: %w", artifact.ID, err)
		}
		if artifact.Path == ManifestPath || artifact.Path == InstructionsPath {
			return fmt.Errorf("manifest artifact %q uses reserved package path %q", artifact.ID, artifact.Path)
		}
		if ids[artifact.ID] {
			return fmt.Errorf("duplicate manifest artifact id %q", artifact.ID)
		}
		if paths[artifact.Path] {
			return fmt.Errorf("duplicate manifest artifact path %q", artifact.Path)
		}
		if !validDigest(artifact.SHA256) {
			return fmt.Errorf("manifest artifact %q has an invalid SHA-256 digest", artifact.ID)
		}
		if err := ValidatePrivacy(artifact.Privacy, artifact.PrivacyClearance); err != nil {
			return fmt.Errorf("manifest artifact %q: %w", artifact.ID, err)
		}
		ids[artifact.ID], paths[artifact.Path] = true, true
	}
	for i, exclusion := range m.Exclusions {
		if strings.TrimSpace(exclusion.ID) == "" || strings.TrimSpace(exclusion.Reason) == "" {
			return fmt.Errorf("manifest exclusion %d requires id and reason", i+1)
		}
	}
	return nil
}

// ValidatePrivacy enforces the package classification and clearance contract.
func ValidatePrivacy(value string, clearance *PrivacyClearance) error {
	switch value {
	case PrivacyNone:
		if clearance != nil {
			return errors.New("privacy clearance is only valid for personal or real data")
		}
		return nil
	case PrivacyPersonal, PrivacyReal:
		if clearance == nil || clearance.Status != PrivacyCleared || strings.TrimSpace(clearance.Basis) == "" {
			return errors.New("personal or real data requires privacy_clearance status cleared and a non-empty basis")
		}
		return nil
	default:
		return fmt.Errorf("privacy must be one of none, personal or real")
	}
}

func validateRelativePath(value string) error {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") || value == ".." {
		return fmt.Errorf("path %q must be a clean package-relative path", value)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("path %q must be a clean package-relative path", value)
		}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}
