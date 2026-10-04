package todo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	reg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("a missing register must not error: %v", err)
	}
	if len(reg.Items) != 0 {
		t.Fatalf("want an empty register, got %d items", len(reg.Items))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := &Register{Items: []Item{
		{ID: "integrate-chatbot", Text: "integrate chatbot", Created: "2026-10-04", Source: "chat"},
		{ID: "second-idea", Text: "second idea", Created: "2026-10-04", Done: true},
	}}
	if err := Save(root, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Items) != 2 || got.Items[0].ID != "integrate-chatbot" || !got.Items[1].Done {
		t.Fatalf("round-trip mismatch: %+v", got.Items)
	}
	if !strings.Contains(string(mustRead(t, Path(root))), "integrate chatbot") {
		t.Error("register file must carry the item text")
	}
}

func TestLoadRejectsMalformed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("items: [oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("want an error for malformed YAML")
	}
}

func TestValidateRejectsInvalid(t *testing.T) {
	cases := map[string]Register{
		"duplicate id": {Items: []Item{
			{ID: "a", Text: "x", Created: "2026-10-04"},
			{ID: "a", Text: "y", Created: "2026-10-04"},
		}},
		"bad id":       {Items: []Item{{ID: "Not A Slug", Text: "x", Created: "2026-10-04"}}},
		"empty text":   {Items: []Item{{ID: "a", Text: "  ", Created: "2026-10-04"}}},
		"bad created":  {Items: []Item{{ID: "a", Text: "x", Created: "04/10/2026"}}},
		"missing date": {Items: []Item{{ID: "a", Text: "x"}}},
	}
	for name, reg := range cases {
		if err := reg.Validate(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestIDForDerivation(t *testing.T) {
	reg := &Register{}
	if got := reg.IDFor("Integrate the Chatbot!", ""); got != "integrate-the-chatbot" {
		t.Errorf("slug derivation = %q", got)
	}
	reg.Add(Item{ID: "integrate-the-chatbot", Text: "x", Created: "2026-10-04"})
	if got := reg.IDFor("Integrate the Chatbot!", ""); got != "integrate-the-chatbot-2" {
		t.Errorf("collision suffix = %q", got)
	}
	if got := reg.IDFor("", "explicit-id"); got != "explicit-id" {
		t.Errorf("explicit id = %q", got)
	}
	if got := reg.IDFor("!!!", ""); got != "item" {
		t.Errorf("empty slug fallback = %q", got)
	}
}

func TestMarkdownProjection(t *testing.T) {
	reg := &Register{Items: []Item{
		{ID: "a", Text: "first idea", Created: "2026-10-04"},
		{ID: "b", Text: "done idea", Created: "2026-10-04", Done: true},
	}}
	md := reg.Markdown()
	for _, want := range []string{"source of truth is context/todo.yaml", "## TODO", "- [ ] first idea", "- [x] done idea"} {
		if !strings.Contains(md, want) {
			t.Errorf("projection missing %q:\n%s", want, md)
		}
	}
	if empty := (&Register{}).Markdown(); !strings.Contains(empty, "_empty_") {
		t.Errorf("empty projection = %q", empty)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
