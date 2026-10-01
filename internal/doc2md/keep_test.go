package doc2md

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempSource(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInspect(t *testing.T) {
	src := tempSource(t, "doc.pdf", "hello\n")
	info, err := Inspect(src)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("hello\n")))
	if info.SHA256 != want {
		t.Errorf("sha = %s, want %s", info.SHA256, want)
	}
	if info.Size != 6 {
		t.Errorf("size = %d", info.Size)
	}
	if info.Digest() != want[:12] {
		t.Errorf("digest = %s", info.Digest())
	}

	if _, err := Inspect(filepath.Join(t.TempDir(), "missing.pdf")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestKeptFileName(t *testing.T) {
	digest := "0123456789ab"
	cases := []struct {
		source string
		name   string
		want   string
	}{
		{"/docs/Report Q3.pdf", "", "report-q3-" + digest + ".md"},
		{"Città àccenti.docx", "", "città-àccenti-" + digest + ".md"},
		{"/docs/Report.pdf", "Custom Name", "custom-name-" + digest + ".md"},
		{"/docs/.pdf", "", "document-" + digest + ".md"},
	}
	for _, tc := range cases {
		if got := KeptFileName(tc.source, tc.name, digest); got != tc.want {
			t.Errorf("KeptFileName(%q, %q) = %q, want %q", tc.source, tc.name, got, tc.want)
		}
	}
}

func TestRenderProvenance(t *testing.T) {
	src := tempSource(t, "report.csv", "a,b\n1,2\n")
	now := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	out, err := Render(Document{
		Source:    src,
		Converter: "anydoc",
		Markdown:  "# My Title\n\nbody",
		Now:       now,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("missing frontmatter fence: %q", text)
	}
	for _, want := range []string{
		"source: " + src,
		"title: My Title",
		"converter: anydoc",
		"converted_at: \"2026-10-01T09:30:00Z\"",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "summary:") || strings.Contains(text, "links:") {
		t.Errorf("unset summary/links should be omitted:\n%s", text)
	}
	if !strings.HasSuffix(text, "# My Title\n\nbody\n") {
		t.Errorf("body not preserved:\n%s", text)
	}
}

func TestRenderSummaryAndLinks(t *testing.T) {
	src := tempSource(t, "report.csv", "a\n")
	out, err := Render(Document{
		Source:    src,
		Converter: "docling",
		Markdown:  "body\n",
		Summary:   "quarterly report",
		Links:     []string{"analysis/20260929-064509-example.md"},
		Now:       time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "summary: quarterly report") {
		t.Errorf("missing summary:\n%s", text)
	}
	if !strings.Contains(text, "- analysis/20260929-064509-example.md") {
		t.Errorf("missing link:\n%s", text)
	}
}

func TestKeepIsIdempotent(t *testing.T) {
	src := tempSource(t, "report.csv", "a,b\n1,2\n")
	dir := filepath.Join(t.TempDir(), "converted")
	doc := Document{Source: src, Converter: "anydoc", Markdown: "# T\n"}

	path, created, err := Keep(doc, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first keep should create the file")
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	path2, created2, err := Keep(doc, dir)
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Error("second keep should not create a new file")
	}
	if path2 != path {
		t.Errorf("path = %q, want %q", path2, path)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("second keep rewrote the file")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected one kept file, got %d", len(entries))
	}
}

func TestKeepPreservesExistingMetadata(t *testing.T) {
	src := tempSource(t, "report.csv", "a\n")
	dir := t.TempDir()
	doc := Document{Source: src, Converter: "anydoc", Markdown: "# T\n"}

	path, _, err := Keep(doc, dir)
	if err != nil {
		t.Fatal(err)
	}
	enriched := strings.Replace(string(mustRead(t, path)), "converted_at:", "summary: enriched\nconverted_at:", 1)
	if err := os.WriteFile(path, []byte(enriched), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Keep(doc, dir); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, path)); got != enriched {
		t.Error("re-keeping overwrote an enriched file")
	}
}

func TestFirstHeading(t *testing.T) {
	cases := []struct {
		markdown string
		want     string
	}{
		{"# Title\nbody", "Title"},
		{"intro\n\n## Not H1\n# Later\n", "Later"},
		{"## only h2\n", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := firstHeading(tc.markdown); got != tc.want {
			t.Errorf("firstHeading(%q) = %q, want %q", tc.markdown, got, tc.want)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
