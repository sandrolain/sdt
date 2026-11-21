package main

import (
	"path/filepath"
	"sort"
	"strings"
)

type Change struct {
	Status string `json:"status"`
	Path   string `json:"path"`
	Old    string `json:"old,omitempty"`
}

type Package struct {
	ImportPath string
	Dir        string
	Imports    []string
}

type Check struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason"`
	Packages []string `json:"packages,omitempty"`
}

type Selection struct {
	Mode             string   `json:"mode"`
	Group            string   `json:"group,omitempty"`
	Base             string   `json:"base,omitempty"`
	Fallback         string   `json:"fallback,omitempty"`
	ChangedFiles     []Change `json:"changed_files,omitempty"`
	SelectedChecks   []Check  `json:"selected_checks"`
	DeferredChecks   []Check  `json:"deferred_checks,omitempty"`
	OutOfScopeChecks []Check  `json:"out_of_scope_checks,omitempty"`
	AgentPackages    []string `json:"agent_added_packages,omitempty"`
}

type checkSpec struct {
	name  string
	group string
}

var fullChecks = []checkSpec{
	{name: "format", group: "test"},
	{name: "modules", group: "test"},
	{name: "entry-metadata", group: "test"},
	{name: "delivery-tool-test", group: "test"},
	{name: "build", group: "build"},
	{name: "vet", group: "test"},
	{name: "lint", group: "lint"},
	{name: "test", group: "test"},
	{name: "coverage", group: "test"},
	{name: "web", group: "web"},
	{name: "govulncheck", group: "security"},
	{name: "gosec", group: "security"},
	{name: "trivy", group: "security"},
}

func selectChecks(root, module, base string, changes []Change, packages []Package, addedPackages []string, full bool) Selection {
	selection := Selection{Mode: "targeted", Base: base, ChangedFiles: changes}
	goPackages := map[string]string{}
	web := false
	contextDocs := false
	entryMetadata := false
	fallback := ""

	if full {
		selection.Mode = "full"
		selection.SelectedChecks = checksForAll("required by explicit full mode")
		return selection
	}
	if base == "" {
		fallback = "no comparable merge base could be established"
	}
	if len(changes) == 0 && fallback == "" {
		fallback = "no changed paths were found"
	}

	for _, change := range changes {
		for _, changedPath := range []string{change.Old, change.Path} {
			if changedPath == "" {
				continue
			}
			clean := filepath.ToSlash(filepath.Clean(changedPath))
			if isFullFallbackPath(clean) {
				fallback = "shared build, dependency, gate, or CI configuration changed: " + clean
				continue
			}
			if strings.HasPrefix(clean, "context/instructions/") || strings.HasPrefix(clean, "context/roles/") {
				fallback = "agent instruction or generated role changed: " + clean
				continue
			}
			if strings.HasPrefix(clean, "context/") {
				contextDocs = true
				continue
			}
			if strings.HasPrefix(clean, "web/") {
				web = true
				continue
			}
			if clean == "main.go" || clean == "cli/main.go" || clean == "viewer/main.go" {
				entryMetadata = true
			}
			if strings.HasPrefix(clean, "internal/acpbridge/testdata/wire/") {
				web = true
			}
			pkg := packageForPath(root, clean, packages)
			if pkg == "" {
				fallback = "changed path has no declared check mapping: " + clean
				continue
			}
			goPackages[pkg] = "changed path: " + clean
		}
	}
	if fallback != "" {
		selection.Mode = "full-fallback"
		selection.Fallback = fallback
		selection.SelectedChecks = checksForAll("full fallback: " + fallback)
		return selection
	}

	for _, pkg := range reverseDependencyClosure(goPackages, packages) {
		if _, selected := goPackages[pkg]; !selected {
			goPackages[pkg] = "reverse dependency of a changed Go package"
		}
	}
	for _, requested := range addedPackages {
		if pkg := resolvePackageName(requested, module, packages); pkg != "" {
			goPackages[pkg] = "added by the agent"
			selection.AgentPackages = append(selection.AgentPackages, requested)
		}
	}
	sort.Strings(selection.AgentPackages)

	if len(goPackages) > 0 {
		packageDirs := make([]string, 0, len(goPackages))
		for importPath := range goPackages {
			if path, ok := relativePackage(importPath, module); ok {
				packageDirs = append(packageDirs, path)
			}
		}
		sort.Strings(packageDirs)
		for _, name := range []string{"format", "build", "vet", "lint", "test", "gosec"} {
			selection.SelectedChecks = append(selection.SelectedChecks, Check{
				Name: name, Status: "selected", Reason: "changed Go packages and reverse dependencies", Packages: packageDirs,
			})
		}
		if entryMetadata {
			selection.SelectedChecks = append(selection.SelectedChecks, Check{Name: "entry-metadata", Status: "selected", Reason: "Go entry point changed"})
		}
	}
	if web {
		selection.SelectedChecks = append(selection.SelectedChecks, Check{Name: "web", Status: "selected", Reason: "web files or ACP wire fixtures changed"})
		if viewer := packagePathForSuffix(packages, "/viewer"); viewer != "" {
			selection.SelectedChecks = append(selection.SelectedChecks, Check{Name: "build", Status: "selected", Reason: "web assets are embedded by the viewer", Packages: []string{viewer}})
			selection.SelectedChecks = append(selection.SelectedChecks, Check{Name: "test", Status: "selected", Reason: "web assets are embedded by the viewer", Packages: []string{viewer}})
		}
	}
	if contextDocs {
		selection.SelectedChecks = append(selection.SelectedChecks, Check{Name: "context-lint", Status: "selected", Reason: "SDT corpus documents changed"})
	}
	if len(selection.SelectedChecks) == 0 {
		selection.Mode = "full-fallback"
		selection.Fallback = "no check was selected for the changed scope"
		selection.SelectedChecks = checksForAll("full fallback: " + selection.Fallback)
		return selection
	}

	selected := map[string]bool{}
	for _, check := range selection.SelectedChecks {
		selected[check.Name] = true
	}
	for _, spec := range fullChecks {
		if selected[spec.name] {
			continue
		}
		selection.DeferredChecks = append(selection.DeferredChecks, Check{
			Name: spec.name, Status: "deferred_to_ci", Reason: "not selected for this local change; CI runs the full check set",
		})
	}
	if !selected["context-lint"] {
		selection.OutOfScopeChecks = append(selection.OutOfScopeChecks, Check{
			Name: "context-lint", Status: "out_of_scope", Reason: "no SDT corpus documents changed",
		})
	}
	return selection
}

func checksForAll(reason string) []Check {
	checks := make([]Check, 0, len(fullChecks))
	for _, spec := range fullChecks {
		checks = append(checks, Check{Name: spec.name, Status: "selected", Reason: reason})
	}
	return checks
}

func hasCheck(checks []Check, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return true
		}
	}
	return false
}

func isFullFallbackPath(path string) bool {
	for _, exact := range []string{".sdt.yaml", "Taskfile.yml", "Taskfile.yaml", "go.mod", "go.sum", ".tool-versions", "scripts/verify.sh", "scripts/verify-delivery.sh", ".github", "AGENTS.md"} {
		if path == exact || (exact == ".github" && strings.HasPrefix(path, ".github/")) {
			return true
		}
	}
	if strings.HasPrefix(path, "scripts/") || strings.HasPrefix(path, ".agents/") || strings.HasPrefix(path, "internal/templates/") {
		return true
	}
	return false
}

func packageForPath(root, changed string, packages []Package) string {
	abs := filepath.Join(root, filepath.FromSlash(changed))
	repoRel, err := filepath.Rel(root, abs)
	if err != nil || (filepath.Dir(repoRel) == "." && filepath.Ext(repoRel) != ".go") {
		return ""
	}
	best := ""
	bestLen := -1
	for _, pkg := range packages {
		if filepath.Ext(abs) == ".go" && filepath.Dir(abs) != pkg.Dir {
			continue
		}
		rel, err := filepath.Rel(pkg.Dir, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if len(pkg.Dir) > bestLen {
			best, bestLen = pkg.ImportPath, len(pkg.Dir)
		}
	}
	return best
}

func reverseDependencyClosure(initial map[string]string, packages []Package) []string {
	selected := make(map[string]bool, len(initial))
	for pkg := range initial {
		selected[pkg] = true
	}
	changed := true
	for changed {
		changed = false
		for _, pkg := range packages {
			if selected[pkg.ImportPath] {
				continue
			}
			for _, imported := range pkg.Imports {
				if selected[imported] {
					selected[pkg.ImportPath] = true
					changed = true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(selected))
	for pkg := range selected {
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

func packageExists(importPath string, packages []Package) bool {
	for _, pkg := range packages {
		if pkg.ImportPath == importPath {
			return true
		}
	}
	return false
}

func resolvePackageName(requested, module string, packages []Package) string {
	name := strings.TrimSpace(requested)
	if name == "." || name == "./" {
		name = module
	} else {
		name = strings.TrimPrefix(name, "./")
	}
	for _, pkg := range packages {
		if pkg.ImportPath == name || strings.TrimPrefix(pkg.ImportPath, module+"/") == name {
			return pkg.ImportPath
		}
	}
	return ""
}

func relativePackage(importPath, module string) (string, bool) {
	if importPath == module {
		return ".", true
	}
	prefix := module + "/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	return "./" + strings.TrimPrefix(importPath, prefix), true
}

func packagePathForSuffix(packages []Package, suffix string) string {
	for _, pkg := range packages {
		if strings.HasSuffix(pkg.ImportPath, suffix) {
			if path, ok := relativePackage(pkg.ImportPath, modulePath(packages)); ok {
				return path
			}
		}
	}
	return ""
}

func modulePath(packages []Package) string {
	for _, pkg := range packages {
		if !strings.Contains(strings.TrimPrefix(pkg.ImportPath, "github.com/"), "/") {
			return pkg.ImportPath
		}
	}
	// The module root is the shortest import path in this product package set.
	module := ""
	for _, pkg := range packages {
		if module == "" || len(pkg.ImportPath) < len(module) {
			module = pkg.ImportPath
		}
	}
	return module
}
