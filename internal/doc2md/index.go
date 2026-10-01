package doc2md

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// IndexEntry is one row of the converted-corpus index, projected from the
// per-file frontmatter.
type IndexEntry struct {
	File        string   `json:"file" yaml:"file"`
	Source      string   `json:"source" yaml:"source"`
	SHA256      string   `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Converter   string   `json:"converter,omitempty" yaml:"converter,omitempty"`
	ConvertedAt string   `json:"converted_at,omitempty" yaml:"converted_at,omitempty"`
	Summary     string   `json:"summary,omitempty" yaml:"summary,omitempty"`
	Links       []string `json:"links,omitempty" yaml:"links,omitempty"`
}

// Index is the whole projection, as emitted to index.json.
type Index struct {
	Documents []IndexEntry `json:"documents" yaml:"documents"`
}

const (
	// IndexMarkdownName and IndexJSONName are the two projections written
	// beside the kept documents.
	IndexMarkdownName = "index.md"
	IndexJSONName     = "index.json"
)

// ReadIndex scans dir for kept documents and projects their frontmatter. It
// reads the files, never the index, so the projection can never become a
// second source of truth. A missing directory is an empty index.
func ReadIndex(dir string) (Index, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Index{}, nil
		}
		return Index{}, fmt.Errorf("read %s: %w", dir, err)
	}

	idx := Index{Documents: []IndexEntry{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == IndexMarkdownName {
			continue
		}
		//#nosec G304 -- scanning the corpus directory
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return Index{}, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		idx.Documents = append(idx.Documents, parseIndexEntry(e.Name(), string(data)))
	}
	sort.Slice(idx.Documents, func(i, j int) bool {
		return idx.Documents[i].File < idx.Documents[j].File
	})
	return idx, nil
}

type indexedFrontmatter struct {
	Source      string   `yaml:"source"`
	SHA256      string   `yaml:"sha256"`
	Converter   string   `yaml:"converter"`
	ConvertedAt string   `yaml:"converted_at"`
	Summary     string   `yaml:"summary"`
	Links       []string `yaml:"links"`
}

func parseIndexEntry(file, content string) IndexEntry {
	entry := IndexEntry{File: file}
	fm := splitFrontmatterBlock(content)
	if fm == "" {
		return entry
	}
	var parsed indexedFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
		return entry
	}
	entry.Source = parsed.Source
	entry.SHA256 = parsed.SHA256
	entry.Converter = parsed.Converter
	entry.ConvertedAt = parsed.ConvertedAt
	entry.Summary = parsed.Summary
	entry.Links = parsed.Links
	return entry
}

// splitFrontmatterBlock returns the YAML text between the leading "---" fences,
// or "" when there is none.
func splitFrontmatterBlock(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return ""
	}
	rest := normalized[len("---\n"):]
	if end := strings.Index(rest, "\n---"); end >= 0 {
		return rest[:end]
	}
	return ""
}

// RenderMarkdown renders the index.md projection with the generation contract
// in its header, so a hand edit is visibly wrong.
func RenderMarkdown(idx Index) string {
	var b strings.Builder
	b.WriteString("## Converted documents\n\n")
	b.WriteString("_Managed by `sdt doc2md reindex`. One row per kept document; the per-file frontmatter is the source._\n\n")
	b.WriteString("| File | Source | sha256 | Converter | Converted | Summary | Links |\n")
	b.WriteString("|------|--------|--------|-----------|-----------|---------|-------|\n")
	for _, e := range idx.Documents {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			mdCell(e.File), mdCell(e.Source), mdCell(shortDigest(e.SHA256)), mdCell(e.Converter),
			mdCell(e.ConvertedAt), mdCell(e.Summary), mdCell(strings.Join(e.Links, ", ")))
	}
	return b.String()
}

// RenderJSON renders the index.json projection.
func RenderJSON(idx Index) ([]byte, error) {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func shortDigest(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

// Reindex rebuilds both projections from the kept files, writing neither when
// unchanged (so the if-changed guard covers both artifacts).
func Reindex(dir string) (wroteMD, wroteJSON bool, err error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return false, false, err
	}
	md := []byte(RenderMarkdown(idx))
	jsonData, err := RenderJSON(idx)
	if err != nil {
		return false, false, err
	}
	wroteMD, err = writeIfChanged(filepath.Join(dir, IndexMarkdownName), md)
	if err != nil {
		return wroteMD, false, err
	}
	wroteJSON, err = writeIfChanged(filepath.Join(dir, IndexJSONName), jsonData)
	return wroteMD, wroteJSON, err
}

func writeIfChanged(path string, data []byte) (bool, error) {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) { //#nosec G304 -- corpus path
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //#nosec G301 -- corpus directory
		return false, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil { //#nosec G306 -- corpus markdown/json
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// SetMetadata enriches a kept document's frontmatter (summary and links) and
// refreshes both projections. It is the only mutator of a kept document.
func SetMetadata(dir, file, summary string, links []string) error {
	path := filepath.Join(dir, filepath.Base(file))
	//#nosec G304 G703 -- the file names a document inside the corpus directory
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	entry := parseIndexEntry(file, string(data))
	if summary != "" {
		entry.Summary = summary
	}
	if len(links) > 0 {
		entry.Links = links
	}

	body := splitBody(string(data))
	fm, err := yaml.Marshal(indexedFrontmatter{
		Source:      entry.Source,
		SHA256:      entry.SHA256,
		Converter:   entry.Converter,
		ConvertedAt: entry.ConvertedAt,
		Summary:     entry.Summary,
		Links:       entry.Links,
	})
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}
	content := "---\n" + string(fm) + "---\n\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G306 G703 -- corpus markdown
		return fmt.Errorf("write %s: %w", file, err)
	}
	_, _, err = Reindex(dir)
	return err
}

// splitBody returns the markdown body after the frontmatter block.
func splitBody(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return normalized
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return normalized
	}
	body := rest[end+len("\n---"):]
	return strings.TrimPrefix(body, "\n")
}

// Now is injectable for deterministic tests.
var Now = func() time.Time { return time.Now().UTC() }
