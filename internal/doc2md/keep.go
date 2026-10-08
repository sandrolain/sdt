package doc2md

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/goccy/go-yaml"
)

// SourceInfo is the content-addressed identity of an input document.
type SourceInfo struct {
	Path   string
	Size   int64
	SHA256 string
}

// Digest returns the short (12 hex) digest used in kept filenames.
func (s SourceInfo) Digest() string {
	if len(s.SHA256) < 12 {
		return s.SHA256
	}
	return s.SHA256[:12]
}

// Inspect hashes an input document without loading it into memory.
func Inspect(path string) (SourceInfo, error) {
	//#nosec G304 -- the path is the input document named on the command line
	f, err := os.Open(path)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only handle

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("hash %s: %w", path, err)
	}
	return SourceInfo{Path: path, Size: size, SHA256: fmt.Sprintf("%x", h.Sum(nil))}, nil
}

// Document is one conversion destined to be kept or written to a file.
type Document struct {
	Source    string
	Name      string
	Converter string
	Markdown  string
	Summary   string
	Links     []string
	Now       time.Time
}

// provenance is the per-file frontmatter of a kept conversion. It mirrors the
// crawldown capture shape (a source identity, a title, a timestamp) so a refs/
// reader sees one convention across web captures and converted documents.
type provenance struct {
	Source      string   `yaml:"source"`
	Title       string   `yaml:"title,omitempty"`
	SHA256      string   `yaml:"sha256"`
	Converter   string   `yaml:"converter"`
	ConvertedAt string   `yaml:"converted_at"`
	Summary     string   `yaml:"summary,omitempty"`
	Links       []string `yaml:"links,omitempty"`
}

// Render builds the provenance-stamped markdown for doc.
func Render(doc Document) ([]byte, error) {
	info, err := Inspect(doc.Source)
	if err != nil {
		return nil, err
	}
	return renderWith(info, doc)
}

func renderWith(info SourceInfo, doc Document) ([]byte, error) {
	now := doc.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summary := doc.Summary
	if summary == "" {
		summary = firstParagraph(doc.Markdown)
	}
	prov := provenance{
		Source:      doc.Source,
		Title:       firstHeading(doc.Markdown),
		SHA256:      info.SHA256,
		Converter:   doc.Converter,
		ConvertedAt: now.UTC().Format(time.RFC3339),
		Summary:     summary,
		Links:       doc.Links,
	}
	data, err := yaml.Marshal(prov)
	if err != nil {
		return nil, fmt.Errorf("marshal provenance: %w", err)
	}
	body := doc.Markdown
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return []byte("---\n" + string(data) + "---\n\n" + body), nil
}

// KeptFileName returns the canonical kept filename <stem>-<digest>.md. An
// explicit name overrides the source stem.
func KeptFileName(source, name, digest string) string {
	stem := name
	if stem == "" {
		stem = sourceStem(source)
	}
	return fmt.Sprintf("%s-%s.md", slugify(stem), digest)
}

// Keep writes doc under dir with its canonical content-addressed filename.
// Keeping the same source bytes again is idempotent: the existing file is left
// untouched (so a summary or links added later are not lost) and created is
// false.
func Keep(doc Document, dir string) (path string, created bool, err error) {
	info, err := Inspect(doc.Source)
	if err != nil {
		return "", false, err
	}
	path = filepath.Join(dir, KeptFileName(doc.Source, doc.Name, info.Digest()))
	if _, statErr := os.Stat(path); statErr == nil {
		return path, false, nil
	}
	body, err := renderWith(info, doc)
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //#nosec G301 -- corpus directory
		return "", false, fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil { //#nosec G306 -- corpus markdown
		return "", false, fmt.Errorf("write %s: %w", path, err)
	}
	return path, true, nil
}

func firstHeading(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// firstParagraph returns the first non-heading, non-blank, non-fence line of the
// markdown, trimmed — a cheap self-describing summary for a kept conversion. It
// is only used when the caller supplied no summary.
func firstParagraph(markdown string) string {
	fence := false
	for _, line := range strings.Split(markdown, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if fence || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return t
	}
	return ""
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "document"
}
