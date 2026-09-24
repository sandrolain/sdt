package templates

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// parseFuncs supplies the func names referenced by dynamic templates built in
// later migration phases; parse-only validation needs them defined. Stubs are
// replaced by the real funcMap in cmd's render path, never here.
var parseFuncs = template.FuncMap{}

// TestEmbeddedTemplatesParse parses every embedded .tmpl file. A parse error
// means a template is syntactically broken (action syntax, unknown func).
func TestEmbeddedTemplatesParse(t *testing.T) {
	seen := 0
	err := fs.WalkDir(templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".tmpl" {
			return nil
		}
		seen++
		if _, perr := template.New(filepath.Base(path)).Funcs(parseFuncs).ParseFS(templatesFS, path); perr != nil {
			t.Errorf("parse %s: %v", path, perr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded templates: %v", err)
	}
	if seen == 0 {
		t.Fatal("no embedded templates found")
	}
}

// TestTemplateRenderRoundTrip guards the static-template guarantee: rendering
// a template with no actions and nil data returns its file content unchanged.
func TestTemplateRenderRoundTrip(t *testing.T) {
	names := []string{"workspace/topics.yaml.tmpl"}
	for _, name := range names {
		want, err := templatesFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		got, err := Render(name, nil, nil)
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		if got != string(want) {
			t.Errorf("%s changed on render:\ngot:\n%q\nwant:\n%q", name, got, want)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("%s must end with a trailing newline", name)
		}
	}
}