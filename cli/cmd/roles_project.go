package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/templates"
)

// ── project-layer facts (Wave 3 phase 4) ─────────────────────────────────────
//
// The project layer of every role profile is specialized from repo evidence:
// the AGENTS.md project block, context/instructions/project.md, the top-level
// layout and the real build/test/lint commands. Everything below is
// deterministic and never invents: a fact that cannot be derived becomes a
// tagged assumption surfaced in the profile's "Open assumptions" footer, or a
// ≤3-question user answer tagged (user).

// Source tags carried by generated project-layer lines.
const (
	roleTagRepo       = "(repo)"
	roleTagUser       = "(user)"
	roleTagAssumption = "(assumption)"
)

// roleProjectFacts is the project-derived input to the role project layers,
// produced once per init run so every profile shares the same facts.
type roleProjectFacts struct {
	Stack       string
	Build       string
	Test        string
	Lint        string
	Layout      []string
	Conventions []string
	OwnedPaths  map[string][]string
	UserAnswers []string
	Assumptions []string
}

// roleRepoAreaCandidates maps each register slug to the top-level repo paths the
// role plausibly owns. A path becomes an owned fact only when the directory
// actually exists in the repo layout (derive, never invent); a profile with no
// candidate present simply lists no owned repo paths.
var roleRepoAreaCandidates = map[string][]string{
	roleSlugBackend:  {"cli", "internal"},
	roleSlugFrontend: {"web"},
	roleSlugQA:       {gateStepTest},
	roleSlugDevops:   {".github", "scripts"},
	cmdDocs:          {cmdDocs, ctxTypeWiki},
}

// projectBlockHeadings canonicalizes the write-once project block sections.
var projectBlockHeadings = map[string]string{
	"Stack":         "stack",
	"Build & Run":   "build",
	"Test":          gateStepTest,
	"Lint & Format": gateStepLint,
	"Conventions":   "conventions",
}

// deriveRoleProjectFacts scans the current directory for repo evidence and
// returns the facts shared by all role profiles. Deterministic: reads only, no
// prompts, no side effects. Facts that cannot be derived are recorded in
// Assumptions so profiles never present an invented rule as fact.
func deriveRoleProjectFacts() roleProjectFacts {
	f := roleProjectFacts{OwnedPaths: map[string][]string{}}
	f.Layout = repoTopLevelDirs()

	proj := agentsProjectBlockSections()
	f.Stack = cleanProjectSection(proj["stack"])
	f.Build = cleanProjectSection(proj["build"])
	f.Test = cleanProjectSection(proj[gateStepTest])
	f.Lint = cleanProjectSection(proj[gateStepLint])
	f.Conventions = cleanProjectLines(proj["conventions"])

	// Prefer the human-verified AGENTS.md block; fall back to the task system.
	if f.Build == "" {
		if c, ok := inferProjectCommand("build"); ok {
			f.Build = c
		}
	}
	if f.Test == "" {
		if c, ok := inferProjectCommand(gateStepTest); ok {
			f.Test = c
		}
	}
	if f.Lint == "" {
		if c, ok := inferProjectCommand(gateStepLint); ok {
			f.Lint = c
		}
	}
	if f.Stack == "" {
		if c, ok := inferStack(); ok {
			f.Stack = c
		}
	}

	for slug, cands := range roleRepoAreaCandidates {
		f.OwnedPaths[slug] = existingRepoDirs(cands)
	}

	f.Assumptions = projectAssumptions(f)
	return f
}

// roleLayoutIgnoredDirs are top-level directories that are SDT-managed, build
// artifacts or tooling, not repo layout facts for a role profile — and that
// `sdt agent roles init` itself creates (context/), which would otherwise make
// the layout scan non-deterministic between runs.
var roleLayoutIgnoredDirs = map[string]bool{
	"context":      true, // SDT workspace, created by this command
	"bin":          true,
	"dist":         true,
	"node_modules": true,
	"vendor":       true,
	"tmp":          true,
	"refs":         true,
}

// repoTopLevelDirs returns the visible top-level directories of the current
// directory (dot-directories and ignored dirs are excluded).
func repoTopLevelDirs() []string {
	entries, err := os.ReadDir(".") //#nosec G304 -- cwd layout scan
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !roleLayoutIgnoredDirs[e.Name()] {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

// existingRepoDirs returns the candidates that exist as directories, in
// candidate order.
func existingRepoDirs(cands []string) []string {
	var out []string
	for _, d := range cands {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() { //#nosec G304 -- fixed candidate dirs
			out = append(out, d)
		}
	}
	return out
}

// agentsProjectBlockSections parses the write-once project block of AGENTS.md
// into its canonical section keys (stack/build/test/lint/conventions). Markers
// are matched on their own line only, so a backtick-quoted reference to the
// block inside the instructions text is never mistaken for the real block.
// HTML comments (fill-in placeholders) are dropped so an untouched template
// yields empty sections, not invented facts.
func agentsProjectBlockSections() map[string]string {
	data, err := os.ReadFile(agentTargetDefault) //#nosec G304 -- fixed repo file
	if err != nil {
		return nil
	}
	begin := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sectionBeginMarker(agentSectionNameProject)) + `\s*$`)
	end := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sectionEndMarker(agentSectionNameProject)) + `\s*$`)
	b := begin.FindIndex(data)
	if b == nil {
		return nil
	}
	e := end.FindIndex(data[b[1]:])
	if e == nil {
		return nil
	}
	block := string(data[b[1] : b[1]+e[0]])

	sections := map[string]string{}
	var key string
	var lines []string
	flush := func() {
		if key != "" {
			sections[key] = strings.Join(lines, "\n")
			lines = nil
		}
	}
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "### ") {
			flush()
			key = projectBlockHeadings[strings.TrimPrefix(t, "### ")]
			continue
		}
		if key != "" {
			lines = append(lines, t)
		}
	}
	flush()
	return sections
}

// cleanProjectSection collapses a project-block section into one compact line,
// dropping HTML comments, code fences, bullets and blank lines.
func cleanProjectSection(s string) string {
	return strings.Join(cleanProjectLines(s), "; ")
}

// cleanProjectLines returns the non-empty, comment-free, fence-free lines of a
// project-block section, with bullets stripped.
func cleanProjectLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(strings.TrimLeft(l, "- "))
		if t == "" || strings.HasPrefix(t, "<!-") || strings.HasPrefix(t, "```") {
			continue
		}
		out = append(out, strings.TrimSpace(t))
	}
	return out
}

// inferProjectCommand derives the real build/test/lint command from the task
// system: Taskfile.yml tasks > web/package.json scripts > Makefile targets.
func inferProjectCommand(name string) (string, bool) {
	if c, ok := taskfileCommand(name); ok {
		return c, true
	}
	if c, ok := packageJSONCommand("web", name); ok {
		return "bun run " + c, true
	}
	if c, ok := makefileCommand(name); ok {
		return c, true
	}
	return "", false
}

// taskfileCommand returns "task <name>" when Taskfile.yml declares the task.
func taskfileCommand(name string) (string, bool) {
	data, err := os.ReadFile("Taskfile.yml") //#nosec G304 -- fixed repo file
	if err != nil {
		return "", false
	}
	var root struct {
		Tasks map[string]struct{} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", false
	}
	if _, ok := root.Tasks[name]; ok {
		return "task " + name, true
	}
	return "", false
}

// packageJSONCommand returns the script value for name in dir/package.json,
// if present.
func packageJSONCommand(dir, name string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json")) //#nosec G304 -- fixed repo file
	if err != nil {
		return "", false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}
	if v, ok := pkg.Scripts[name]; ok {
		return v, true
	}
	return "", false
}

// makefileCommand returns "make <name>" when the Makefile declares the target.
func makefileCommand(name string) (string, bool) {
	data, err := os.ReadFile("Makefile") //#nosec G304 -- fixed repo file
	if err != nil {
		return "", false
	}
	re := regexp.MustCompile(`(?m)^\s*` + name + `\s*:`)
	if re.Match(data) {
		return "make " + name, true
	}
	return "", false
}

// inferStack derives a compact stack line from .tool-versions and go.mod when
// the AGENTS.md project block does not carry a Stack section.
func inferStack() (string, bool) {
	var parts []string
	if data, err := os.ReadFile(".tool-versions"); err == nil { //#nosec G304 -- fixed repo file
		for _, l := range strings.Split(string(data), "\n") {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			f := strings.Fields(l)
			if len(f) == 2 {
				parts = append(parts, f[0]+" "+f[1])
			}
		}
	}
	if _, err := os.Stat("go.mod"); err == nil { //#nosec G304 -- fixed repo file
		parts = append(parts, "Go (go.mod)")
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " · "), true
}

// projectAssumptions records the facts that could not be derived, so profiles
// surface them instead of inventing. A section that exists but only holds
// fill-in comments already produced an empty value by cleanProjectSection.
func projectAssumptions(f roleProjectFacts) []string {
	var a []string
	if f.Stack == "" {
		a = append(a, "stack not derivable (add the AGENTS.md project block `### Stack` or .tool-versions/go.mod)")
	}
	if f.Build == "" {
		a = append(a, "build command not derivable (add `### Build & Run` to the project block or a Taskfile/Makefile/package.json build)")
	}
	if f.Test == "" {
		a = append(a, "test command not derivable (add `### Test` to the project block or a Taskfile/Makefile/package.json test)")
	}
	if f.Lint == "" {
		a = append(a, "lint command not derivable (add `### Lint & Format` to the project block or a Taskfile/Makefile/package.json lint)")
	}
	return a
}

// ── questionnaire (≤3 non-derivable role facts) ───────────────────────────────

// roleQuestionnaireItems are the bounded, non-derivable facts a repo scan
// cannot answer. Collected only with --ask on an interactive terminal; skipped
// answers are recorded as tagged assumptions, never invented.
var roleQuestionnaireItems = []struct {
	key, label string
}{
	{"review", "Independent review pairing — who reviews frontend/backend changes?"},
	{"deploy", "Deployment/release constraints — targets, cadence, environments?"},
	{"other", "Role-relevant conventions or constraints not derivable from the repo?"},
}

// applyRoleAnswers merges non-empty questionnaire answers into the facts,
// tagged (user). Questions answered with an empty value are skipped.
func applyRoleAnswers(f roleProjectFacts, answers []string) roleProjectFacts {
	for i, a := range answers {
		if i >= len(roleQuestionnaireItems) || strings.TrimSpace(a) == "" {
			continue
		}
		f.UserAnswers = append(f.UserAnswers,
			strings.TrimSuffix(roleQuestionnaireItems[i].label, "?")+" → "+strings.TrimSpace(a))
	}
	return f
}

// ── project-layer rendering ────────────────────────────────────────────────────

// roleProjectLayerData is the render input for roles/project-layer.md.tmpl:
// the register slug plus the per-role slice of the derived facts.
type roleProjectLayerData struct {
	Slug        string
	Stack       string
	Build       string
	Test        string
	Lint        string
	Layout      []string
	OwnedPaths  []string
	Conventions []string
	UserAnswers []string
	Assumptions []string
}

// roleProjectLayer renders the `roles/<slug>/project` marker body from the
// derived facts, with every generated line tagged by source. It replaces the
// phase-2 stub once project derivation ships.
func roleProjectLayer(r roleDescriptor, f roleProjectFacts) string {
	data := roleProjectLayerData{
		Slug:        r.Slug,
		Stack:       f.Stack,
		Build:       f.Build,
		Test:        f.Test,
		Lint:        f.Lint,
		Layout:      f.Layout,
		OwnedPaths:  f.OwnedPaths[r.Slug],
		Conventions: f.Conventions,
		UserAnswers: f.UserAnswers,
		Assumptions: f.Assumptions,
	}
	return templates.Must("roles/project-layer.md.tmpl", data, roleFuncMap())
}

// tagged renders a fact line with (repo) when derived, (assumption) when not.
func tagged(fact, repoTag, assumptionTag string) string {
	if strings.TrimSpace(fact) == "" {
		return "not derivable " + assumptionTag
	}
	return fact + " " + repoTag
}
