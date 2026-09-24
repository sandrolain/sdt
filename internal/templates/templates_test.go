package templates

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// parseFuncs supplies the func names referenced by dynamic templates built in
// later migration phases; parse-only validation needs them defined. Stubs are
// replaced by the real funcMap in cmd's render path, never here.
var parseFuncs = template.FuncMap{
	"yesNo":     func(bool) string { return "" },
	"ownedList": func([]string) string { return "" },
	"join":      func([]string) string { return "" },
	"tagged":    func(string) string { return "" },
}

// renderCases holds the representative data blob for each dynamic template so
// the parse guard below can also *execute* every template. Static templates
// (no actions) render with nil data and are covered here too. Missing or
// wrong-typed fields fail execution, so no action is skipped silently.
var renderCases = map[string]any{
	"agents/instructions.md.tmpl": map[string]any{
		"Project": "p",
		"Group":   "g",
	},
	"commands/index.md.tmpl": map[string]any{
		"Project":  "p",
		"Now":      "2026-09-24T00:00:00Z",
		"Triggers": []string{"analysis", "ingestion"},
	},
	"commands/stub.md.tmpl": map[string]any{
		"ID":       "ingestion",
		"Contract": "ingestion",
		"Project":  "p",
		"Now":      "2026-09-24T00:00:00Z",
	},
	"instructions/project.md.tmpl": map[string]any{
		"Project": "p",
		"Group":   "g",
	},
	"roles/shared.md.tmpl": map[string]any{
		"Rows": []map[string]any{{
			"Slug":        "pm",
			"Title":       "Project manager",
			"Coordinator": true,
			"Owned":       []string{"context/plan", "context/tasks"},
		}},
	},
	"roles/project-layer.md.tmpl": map[string]any{
		"Slug":        "backend",
		"Stack":       "Go 1.27",
		"Build":       "task build",
		"Test":        "go test ./cli/...",
		"Lint":        "golangci-lint run",
		"Layout":      []string{"cli/cmd", "internal/templates"},
		"OwnedPaths":  []string{"cli/cmd"},
		"Conventions": []string{"table-driven tests"},
		"UserAnswers": []string{"Deploys weekly"},
		"Assumptions": []string{"Uses Makefile"},
	},
	"workspace/readme.md.tmpl": map[string]any{
		"CommandsSection": "## Commands\n(registry-driven)\n",
	},
}

// renderCasesDataFor resolves the sample data for one template path, applying
// the shared blob to every roles/core/<slug>.md.tmpl file.
func renderCasesDataFor(path string) any {
	if d, ok := renderCases[path]; ok {
		return d
	}
	if strings.HasPrefix(path, "roles/core/") {
		return map[string]any{
			"Slug":  "backend",
			"Title": "Backend engineer",
		}
	}
	return nil
}

// TestEmbeddedTemplatesParse parses AND executes every embedded .tmpl file. A
// parse error means the template is syntactically broken (action syntax,
// unknown func); an execution error means a referenced field/func is missing
// or mis-typed — the representative renderCases data exercises every action.
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
		tmpl, perr := template.New(filepath.Base(path)).Funcs(parseFuncs).ParseFS(templatesFS, path)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		var buf bytes.Buffer
		if xerr := tmpl.Execute(&buf, renderCasesDataFor(path)); xerr != nil {
			t.Errorf("execute %s: %v", path, xerr)
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
	static := []string{
		"workspace/topics.yaml.tmpl",
		"workspace/scripts-index.md.tmpl",
		"agents/project.md.tmpl",
		"instructions/analysis.md.tmpl",
		"instructions/architecture.md.tmpl",
		"instructions/decision.md.tmpl",
		"instructions/development.md.tmpl",
		"instructions/git.md.tmpl",
		"instructions/ingestion.md.tmpl",
		"instructions/lessons.md.tmpl",
		"instructions/notes.md.tmpl",
		"instructions/plan.md.tmpl",
		"instructions/prompts.md.tmpl",
		"instructions/proposal.md.tmpl",
		"instructions/questions.md.tmpl",
		"instructions/reference.md.tmpl",
		"instructions/research.md.tmpl",
		"instructions/scripts.md.tmpl",
		"instructions/tasks.md.tmpl",
		"instructions/wiki.md.tmpl",
		"instructions/worklog.md.tmpl",
	}
	for _, name := range static {
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

// TestTemplateExists checks the presence probe used by optional template lookup
// (e.g. role core files keyed by register slug).
func TestTemplateExists(t *testing.T) {
	if !Exists("instructions/plan.md.tmpl") {
		t.Error("expected existing template to be reported present")
	}
	if Exists("instructions/nope.md.tmpl") {
		t.Error("expected missing template to be reported absent")
	}
}

// TestCLITemplateEscapesUserExample checks that the template-escaping example
// embedded in the cli instruction file renders back to a literal {{.user}},
// so init output keeps the illustrative action intact.
func TestCLITemplateEscapesUserExample(t *testing.T) {
	got, err := Render("instructions/cli.md.tmpl", nil, nil)
	if err != nil {
		t.Fatalf("render cli.md.tmpl: %v", err)
	}
	if !strings.Contains(got, `Hi {{.user}}`) {
		t.Errorf("expected literal `Hi {{.user}}` example, got:\n%s", got)
	}
	if strings.Contains(got, `{{"{{"}}`) {
		t.Errorf("escaped action leaked into rendered output:\n%s", got)
	}
}

// TestProjectTemplateIdentity covers the four project/group identity combos.
func TestProjectTemplateIdentity(t *testing.T) {
	cases := []struct {
		name    string
		project string
		group   string
		want    string
	}{
		{"both", "p1", "g1", "# Project\n\n- Project: p1\n- Group: g1\n\nThis project"},
		{"project-only", "p1", "", "# Project\n\n- Project: p1\n\nThis project"},
		{"group-only", "", "g1", "# Project\n- Group: g1\n\nThis project"},
		{"none", "", "", "# Project\n\nThis project"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := struct {
				Project string
				Group   string
			}{Project: tc.project, Group: tc.group}
			got, err := Render("instructions/project.md.tmpl", data, nil)
			if err != nil {
				t.Fatalf("render project.md.tmpl: %v", err)
			}
			if !strings.HasPrefix(got, tc.want) {
				t.Errorf("head mismatch\ngot:  %q\nwant: %q", got[:len(tc.want)], tc.want)
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("project.md.tmpl must end with a trailing newline")
			}
		})
	}
}
