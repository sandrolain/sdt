package templates

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/sandrolain/sdt/internal/contextwiki"
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
		"Project": "p",
		"Now":     "2026-09-24T00:00:00Z",
		"Triggers": []map[string]any{
			{"Trigger": "analysis", "Payload": "subject or scope of the analysis to create or extend"},
			{"Trigger": "scratch", "Payload": "_not declared_"},
		},
	},
	"commands/stub.md.tmpl": map[string]any{
		"ID":       "ingestion",
		"Contract": "ingestion",
		"Payload":  "paths and/or URLs to ingest",
		"Examples": []string{">ingestion: context/refs/example-repo", ">ingestion: https://example.com/doc"},
		"Project":  "p",
		"Now":      "2026-09-24T00:00:00Z",
	},
	"instructions/project.md.tmpl": map[string]any{
		"Project": "p",
		"Group":   "g",
	},
	"instructions/cli.md.tmpl": map[string]any{
		"NewTypes":  "plan|analysis",
		"PathTypes": "plan|analysis",
		"ListTypes": "plan|analysis",
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
		"workspace/categories.yaml.tmpl",
		"workspace/memo.yaml.tmpl",
		"workspace/scripts-index.md.tmpl",
		"agents/project.md.tmpl",
		"instructions/analysis.md.tmpl",
		"instructions/architecture.md.tmpl",
		"instructions/browser.md.tmpl",
		"instructions/browser-tools.md.tmpl",
		"instructions/decision.md.tmpl",
		"instructions/development.md.tmpl",
		"instructions/diagram.md.tmpl",
		"instructions/diagram-mermaid.md.tmpl",
		"instructions/distillation.md.tmpl",
		"instructions/git.md.tmpl",
		"instructions/ingestion.md.tmpl",
		"instructions/learning.md.tmpl",
		"instructions/lessons.md.tmpl",
		"instructions/mindmap.md.tmpl",
		"instructions/mindmap-markmap.md.tmpl",
		"instructions/notes.md.tmpl",
		"instructions/plan.md.tmpl",
		"instructions/prompts.md.tmpl",
		"instructions/proposal.md.tmpl",
		"instructions/questions.md.tmpl",
		"instructions/reference.md.tmpl",
		"instructions/research.md.tmpl",
		"instructions/scope.md.tmpl",
		"instructions/scripts.md.tmpl",
		"instructions/slides.md.tmpl",
		"instructions/slides-marp.md.tmpl",
		"instructions/tasks.md.tmpl",
		"instructions/ui.md.tmpl",
		"instructions/ui-tokens.md.tmpl",
		"instructions/ui-components.md.tmpl",
		"instructions/ui-accessibility.md.tmpl",
		"instructions/ui-taste.md.tmpl",
		"instructions/ui-adapters.md.tmpl",
		"instructions/ui-performance.md.tmpl",
		"instructions/ui-layout.md.tmpl",
		"instructions/ui-motion.md.tmpl",
		"instructions/vector.md.tmpl",
		"instructions/vector-svg.md.tmpl",
		"instructions/vector-tools.md.tmpl",
		"instructions/wiki.md.tmpl",
		"instructions/worklog.md.tmpl",
		"instructions/writing.md.tmpl",
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
	data := map[string]any{
		"NewTypes":  "plan|analysis",
		"PathTypes": "plan|analysis",
		"ListTypes": "plan|analysis",
	}
	got, err := Render("instructions/cli.md.tmpl", data, nil)
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
		{"both", "p1", "g1", "- Project: p1\n- Group: g1\n\nThis project"},
		{"project-only", "p1", "", "- Project: p1\n\nThis project"},
		{"group-only", "", "g1", "- Group: g1\n\nThis project"},
		{"none", "", "", "This project"},
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
			// The module carries corpus frontmatter; the identity block below it
			// is what this test pins.
			_, body := contextwiki.SplitFrontmatter(got)
			body = strings.TrimLeft(body, "\n")
			if !strings.HasPrefix(body, tc.want) {
				t.Errorf("head mismatch\ngot:  %q\nwant: %q", body[:len(tc.want)], tc.want)
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("project.md.tmpl must end with a trailing newline")
			}
		})
	}
}

// viewerTokenRe matches the viewer-independence invariant: no `viewer` /
// `sdtviewer` token, word-boundary matched so `reviewer` and `reviewViewer`
// are not flagged.
var viewerTokenRe = regexp.MustCompile(`\b(viewer|sdtviewer)\b`)

// TestGeneratedSetIsViewerFree enforces the viewer-independence invariant over
// the whole generated set: the instruction/agent/workspace templates, the
// AGENTS.md instructions block and the roles core layer. The project layer and
// this repository's own AGENTS.md project block are host data and excluded.
// Word boundaries keep `reviewer`/`reviewViewer` clean.
func TestGeneratedSetIsViewerFree(t *testing.T) {
	// Surface 1: every embedded template file under the generated top-level dirs.
	for _, dir := range []string{"instructions", "agents", "workspace", "roles"} {
		err := fs.WalkDir(templatesFS, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, rerr := templatesFS.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if m := viewerTokenRe.FindAllString(string(data), -1); m != nil {
				t.Errorf("%s: viewer token(s) %v (the generated set must not know the viewer)", path, m)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}

	// Surfaces 2 and 3 live in the working tree, which is absent on a fresh CI
	// checkout of context/ (gitignored) — skip only the parts that are missing.
	root := filepath.Join("..", "..")
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		re := regexp.MustCompile(`(?ms)^<!-- sdt:begin:instructions -->.*?^<!-- sdt:end:instructions -->`)
		if block := re.FindString(string(data)); block != "" {
			if m := viewerTokenRe.FindAllString(block, -1); m != nil {
				t.Errorf("AGENTS.md instructions block: viewer token(s) %v", m)
			}
		} else {
			t.Error("AGENTS.md: instructions block markers not found")
		}
	}

	// Roles core layer: marker-delimited, so the (repo) project layer that
	// legitimately carries host tokens is excluded.
	rolesDir := filepath.Join(root, "context", "roles")
	if entries, err := os.ReadDir(rolesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
				continue
			}
			data, rerr := os.ReadFile(filepath.Join(rolesDir, e.Name()))
			if rerr != nil {
				t.Fatalf("read roles/%s: %v", e.Name(), rerr)
			}
			slug := strings.TrimSuffix(e.Name(), ".md")
			core, ok := roleCoreRegion(string(data), slug)
			if !ok {
				continue // shared.md and any non-profile file
			}
			if m := viewerTokenRe.FindAllString(core, -1); m != nil {
				t.Errorf("context/roles/%s core layer: viewer token(s) %v", e.Name(), m)
			}
		}
	}
}

// roleCoreRegion extracts the marker-delimited `roles/<slug>/core` body from a
// generated role profile, mirroring the extraction the cmd package uses. It
// reports ok=false when the markers are absent (e.g. the shared rules file).
func roleCoreRegion(content, slug string) (string, bool) {
	begin := "<!-- sdt:begin:roles/" + slug + "/core -->"
	end := "<!-- sdt:end:roles/" + slug + "/core -->"
	bi := strings.Index(content, begin)
	if bi < 0 {
		return "", false
	}
	rest := content[bi+len(begin):]
	ei := strings.Index(rest, end)
	if ei < 0 {
		return "", false
	}
	return rest[:ei], true
}

// backtickRe captures the content of one `backticked` span.
var backtickRe = regexp.MustCompile("`([^`]+)`")

// TestIngestionVocabularyMatchesRegistry pins the ingestion module's `type`
// field row and its relation-verb table to the Go registries that
// `sdt context wiki lint` enforces, so a value added to one surface and not the
// other fails here instead of shipping a page the schema rejects. The donor
// this rule came from shipped exactly this drift — four declarations whose
// union held twelve values and whose intersection held two — undetected.
func TestIngestionVocabularyMatchesRegistry(t *testing.T) {
	rendered, err := Render("instructions/ingestion.md.tmpl", nil, nil)
	if err != nil {
		t.Fatalf("render ingestion.md.tmpl: %v", err)
	}

	if got := rowTokens(t, rendered, "type", 2); !sameTokenSet(got, contextwiki.Types) {
		t.Errorf("ingestion `type` row = %v, contextwiki.Types = %v", got, contextwiki.Types)
	}
	if got := verbTableTokens(t, rendered); !sameTokenSet(got, contextwiki.Verbs) {
		t.Errorf("ingestion relation verbs = %v, contextwiki.Verbs = %v", got, contextwiki.Verbs)
	}
}

// rowTokens returns the backticked tokens of the cell at index col of the first
// table row whose first cell equals firstCell (backticks stripped).
func rowTokens(t *testing.T, content, firstCell string, col int) []string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) > col && strings.Trim(cells[0], "`") == firstCell {
			return backtickTokens(cells[col])
		}
	}
	t.Fatalf("table row %q not found", firstCell)
	return nil
}

// verbTableTokens returns the first-column value of every data row of the table
// headed by `| verb | ... |`.
func verbTableTokens(t *testing.T, content string) []string {
	t.Helper()
	inTable := false
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if inTable {
				break
			}
			continue
		}
		cells := splitTableRow(line)
		if len(cells) == 0 {
			continue
		}
		if !inTable {
			if cells[0] == "verb" {
				inTable = true
			}
			continue
		}
		if strings.Trim(cells[0], "-") == "" { // separator row
			continue
		}
		out = append(out, strings.Trim(cells[0], "`"))
	}
	if len(out) == 0 {
		t.Fatal("relation verb table not found")
	}
	return out
}

// splitTableRow splits a markdown table line into trimmed cells, dropping the
// empty cells produced by the outer pipes.
func splitTableRow(line string) []string {
	parts := strings.Split(strings.TrimSpace(line), "|")
	if len(parts) >= 2 {
		parts = parts[1 : len(parts)-1]
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// backtickTokens returns the content of every `backticked` span in s, in order.
func backtickTokens(s string) []string {
	var out []string
	for _, m := range backtickRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// sameTokenSet reports whether got and want hold the same distinct values.
func sameTokenSet(got []string, want map[string]bool) bool {
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	if len(seen) != len(want) {
		return false
	}
	for w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}

// TestInstructionModulesStartAtH2 enforces the document convention on the
// generated instruction set: a body starts at H2, because the frontmatter `title`
// is the document's title. A module's `# Title` line is that title written a
// second time.
//
// The gate lives here, over the templates that ship with the binary, because the
// generated copies cannot carry it: `context/instructions/` is not a registered
// context kind, so the corpus lint refuses a path there ("unknown context type")
// and the generic no-H1 rule never sees a module. A test over the embedded set is
// therefore the only place this invariant fails closed.
func TestInstructionModulesStartAtH2(t *testing.T) {
	checked := 0
	err := fs.WalkDir(templatesFS, "instructions", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".tmpl" {
			return nil
		}
		data, rerr := templatesFS.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		checked++
		if line, head := firstBodyH1(string(data)); line > 0 {
			t.Errorf("%s:%d: module opens with an H1 title %q — the frontmatter `title` is the title and the body starts at H2", path, line, head)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk instructions: %v", err)
	}
	if checked == 0 {
		t.Fatal("no instruction templates were checked — the walk found nothing")
	}
}

// firstBodyH1 returns the line number and text of the first level-1 heading in a
// template's body, ignoring the frontmatter block and anything inside a fenced
// code block (a shell example legitimately shows `# comment`). Position 0 means
// no H1.
func firstBodyH1(content string) (int, string) {
	inFrontmatter := false
	inFence := false
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "#" || strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "#\t") {
			return i + 1, strings.TrimSpace(line)
		}
	}
	return 0, ""
}
