package converter_test

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/cli/utils/converter"
)

func TestNewConverter(t *testing.T) {
	_, err := converter.NewConverter(converter.Options{})
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}
}

func TestConvert_Basic(t *testing.T) {
	conv, _ := converter.NewConverter(converter.Options{})

	markdown, err := conv.Convert("<h1>Hello</h1><p>World</p>")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if !strings.Contains(markdown, "Hello") {
		t.Errorf("Convert() = %q, want to contain 'Hello'", markdown)
	}
}

func TestConvert_EmptyHTML(t *testing.T) {
	conv, _ := converter.NewConverter(converter.Options{})

	_, err := conv.Convert("")
	if err == nil {
		t.Fatal("Convert('') expected error, got nil")
	}
}

func TestGenerateFilename_Root(t *testing.T) {
	filename := converter.GenerateFilename("https://example.com/")
	if filename != "index.md" {
		t.Errorf("GenerateFilename('/') = %q, want 'index.md'", filename)
	}
}

func TestGenerateFilename_Path(t *testing.T) {
	filename := converter.GenerateFilename("https://example.com/docs/intro")
	if filename != "docs-intro.md" {
		t.Errorf("GenerateFilename('/docs/intro') = %q, want 'docs-intro.md'", filename)
	}
}

func TestGenerateFilename_InvalidURL(t *testing.T) {
	filename := converter.GenerateFilename("://bad url")
	if filename != "index.md" {
		t.Errorf("GenerateFilename(invalid) = %q, want 'index.md'", filename)
	}
}

func TestConvertLinksToLocal(t *testing.T) {
	urlToFile := map[string]string{
		"https://example.com/docs/intro": "docs-intro.md",
	}

	input := "[Introduction](https://example.com/docs/intro)"
	result := converter.ConvertLinksToLocal(input, "https://example.com/", urlToFile)

	if !strings.Contains(result, "docs-intro.md") {
		t.Errorf("ConvertLinksToLocal() = %q, want local file reference", result)
	}
}

func TestConvertLinksToLocal_ExternalUnchanged(t *testing.T) {
	urlToFile := map[string]string{}

	input := "[GitHub](https://github.com/user/repo)"
	result := converter.ConvertLinksToLocal(input, "https://example.com/", urlToFile)

	if result != input {
		t.Errorf("ConvertLinksToLocal() external link changed: got %q, want %q", result, input)
	}
}

func TestConvertLinksToLocal_MailtoUnchanged(t *testing.T) {
	urlToFile := map[string]string{}

	input := "[Email](mailto:user@example.com)"
	result := converter.ConvertLinksToLocal(input, "https://example.com/", urlToFile)

	if result != input {
		t.Errorf("ConvertLinksToLocal() mailto changed: got %q, want %q", result, input)
	}
}

func TestDocumentExtension(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"report.pdf", ".pdf"},
		{"REPORT.PDF", ".pdf"},
		{"https://x.example/docs/file.docx?raw=1", ".docx"},
		{"https://x.example/a/b.xlsx#sheet", ".xlsx"},
		{"notes.md", ".md"},
		{"image.png", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := converter.DocumentExtension(tc.path); got != tc.want {
			t.Errorf("DocumentExtension(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestDocumentExtensionsRegistry(t *testing.T) {
	missing := map[string]bool{".pdf": true, ".docx": true, ".md": true, ".csv": true}
	for _, ext := range converter.DocumentExtensions {
		delete(missing, ext)
	}
	if len(missing) != 0 {
		t.Errorf("registry is missing %v", missing)
	}
}

func TestGenerateAssetFilename(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://x.example/", "index.bin"},
		{"https://x.example", "index.bin"},
		{"https://x.example?file=a.pdf", "index-file-a.pdf.bin"},
		{"https://x.example/docs/report", "docs-report.bin"},
		{"%%%", "download.bin"},
	}
	for _, tc := range cases {
		if got := converter.GenerateAssetFilename(tc.url); got != tc.want {
			t.Errorf("GenerateAssetFilename(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
