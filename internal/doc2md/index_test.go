package doc2md

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedCorpus(t *testing.T, dir string) {
	t.Helper()
	fixed := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	Now = func() time.Time { return fixed }
	t.Cleanup(func() { Now = func() time.Time { return time.Now().UTC() } })

	for _, pair := range []struct{ src, markdown string }{
		{"report.csv", "# Report\n\nbody\n"},
		{"notes.txt", "plain notes\n"},
	} {
		path := filepath.Join(t.TempDir(), pair.src)
		if err := os.WriteFile(path, []byte(pair.src+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Keep(Document{Source: path, Converter: "anydoc", Markdown: pair.markdown, Now: fixed}, dir); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadIndexProjectsFrontmatter(t *testing.T) {
	dir := t.TempDir()
	seedCorpus(t, dir)

	idx, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Documents) != 2 {
		t.Fatalf("documents = %+v", idx.Documents)
	}
	first := idx.Documents[0]
	if !strings.HasSuffix(first.File, ".md") {
		t.Errorf("file = %q", first.File)
	}
	if first.Converter != "anydoc" || first.SHA256 == "" || first.Source == "" {
		t.Errorf("entry = %+v", first)
	}
	if first.ConvertedAt != "2026-10-01T09:00:00Z" {
		t.Errorf("converted_at = %q", first.ConvertedAt)
	}
}

func TestReadIndexMissingDir(t *testing.T) {
	idx, err := ReadIndex(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Documents) != 0 {
		t.Errorf("documents = %+v", idx.Documents)
	}
}

func TestReindexIdempotence(t *testing.T) {
	dir := t.TempDir()
	seedCorpus(t, dir)

	wroteMD, wroteJSON, err := Reindex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !wroteMD || !wroteJSON {
		t.Fatalf("first reindex wrote md=%v json=%v", wroteMD, wroteJSON)
	}
	md, err := os.ReadFile(filepath.Join(dir, IndexMarkdownName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Managed by `sdt doc2md reindex`") {
		t.Errorf("markdown header:\n%s", md)
	}
	if !json.Valid(mustRead(t, filepath.Join(dir, IndexJSONName))) {
		t.Error("index.json is not valid JSON")
	}

	wroteMD, wroteJSON, err = Reindex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if wroteMD || wroteJSON {
		t.Error("second reindex rewrote an unchanged projection")
	}
}

func TestReindexReproducesDeletedArtifacts(t *testing.T) {
	dir := t.TempDir()
	seedCorpus(t, dir)
	if _, _, err := Reindex(dir); err != nil {
		t.Fatal(err)
	}
	mdBefore := mustRead(t, filepath.Join(dir, IndexMarkdownName))
	jsonBefore := mustRead(t, filepath.Join(dir, IndexJSONName))

	if err := os.Remove(filepath.Join(dir, IndexMarkdownName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, IndexJSONName)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Reindex(dir); err != nil {
		t.Fatal(err)
	}
	if string(mustRead(t, filepath.Join(dir, IndexMarkdownName))) != string(mdBefore) {
		t.Error("index.md was not reproduced identically")
	}
	if string(mustRead(t, filepath.Join(dir, IndexJSONName))) != string(jsonBefore) {
		t.Error("index.json was not reproduced identically")
	}
}

func TestSetMetadataEnrichesAndReindexes(t *testing.T) {
	dir := t.TempDir()
	seedCorpus(t, dir)
	if _, _, err := Reindex(dir); err != nil {
		t.Fatal(err)
	}

	idx, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := idx.Documents[0].File
	if err := SetMetadata(dir, file, "my summary", []string{"analysis/x.md"}); err != nil {
		t.Fatal(err)
	}

	idx, err = ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range idx.Documents {
		if e.File == file {
			found = true
			if e.Summary != "my summary" || len(e.Links) != 1 || e.Links[0] != "analysis/x.md" {
				t.Errorf("entry = %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("file %q not found after set", file)
	}

	md := string(mustRead(t, filepath.Join(dir, IndexMarkdownName)))
	if !strings.Contains(md, "my summary") || !strings.Contains(md, "analysis/x.md") {
		t.Errorf("index.md not refreshed:\n%s", md)
	}
	doc := string(mustRead(t, filepath.Join(dir, file)))
	if !strings.Contains(doc, "plain notes") {
		t.Errorf("document body lost after set:\n%s", doc)
	}
	if !strings.HasPrefix(doc, "---\n") || !strings.Contains(doc, "summary: my summary") {
		t.Errorf("document frontmatter not rewritten:\n%s", doc)
	}
}

func TestSetMetadataMissingFile(t *testing.T) {
	if err := SetMetadata(t.TempDir(), "nope.md", "s", nil); err == nil {
		t.Error("expected an error for a missing document")
	}
}

func TestRenderMarkdownEscapesPipes(t *testing.T) {
	idx := Index{Documents: []IndexEntry{{File: "a.md", Summary: "x | y"}}}
	if md := RenderMarkdown(idx); !strings.Contains(md, `x \| y`) {
		t.Errorf("pipe not escaped:\n%s", md)
	}
}

func TestSplitFrontmatterBlock(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"lf", "---\nkind: x\n---\nbody", "kind: x"},
		{"crlf", "---\r\nkind: x\r\n---\r\nbody", "kind: x"},
		{"none", "body only", ""},
		{"unterminated", "---\nkind: x\n", ""},
	}
	for _, tc := range cases {
		if got := splitFrontmatterBlock(tc.content); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestShortDigest(t *testing.T) {
	if got := shortDigest("0123456789abcdef"); got != "0123456789ab" {
		t.Errorf("got = %q", got)
	}
	if got := shortDigest("abc"); got != "abc" {
		t.Errorf("got = %q", got)
	}
}

func TestSplitBody(t *testing.T) {
	content := "---\na: 1\n---\n\n# Title\nbody\n"
	if got := splitBody(content); got != "\n# Title\nbody\n" {
		t.Errorf("got = %q", got)
	}
	if got := splitBody("no frontmatter"); got != "no frontmatter" {
		t.Errorf("got = %q", got)
	}
}
