package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestPreviewMatrix(t *testing.T) {
	all := checkNames(fullChecks)
	goChecks := []string{"build", "format", "gosec", "lint", "test", "vet"}
	cases := []struct {
		name    string
		changes []Change
		want    []string
	}{
		{"go", []Change{{Status: "M", Path: "internal/base/store.go"}}, goChecks},
		{"web", []Change{{Status: "M", Path: "web/src/App.tsx"}}, []string{"build", "test", "web"}},
		{"acp", []Change{{Status: "M", Path: "internal/acpbridge/testdata/wire/session.json"}}, []string{"build", "build", "format", "gosec", "lint", "test", "test", "vet", "web"}},
		{"docs", []Change{{Status: "M", Path: "context/plan/example.md"}}, []string{"context-lint"}},
		{"generated", []Change{{Status: "M", Path: "internal/templates/instructions/development.md.tmpl"}}, all},
		{"config", []Change{{Status: "M", Path: "Taskfile.yml"}}, all},
		{"deletion", []Change{{Status: "D", Path: "internal/removed/pkg.go"}}, all},
		{"rename", []Change{{Status: "R100", Old: "internal/base/store.go", Path: "internal/middle/store.go"}}, goChecks},
		{"untracked", []Change{{Status: "?", Path: "internal/base/new.go"}}, goChecks},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", tc.changes, fixturePackages(), nil, false)
			got := make([]string, 0, len(selection.SelectedChecks))
			for _, check := range selection.SelectedChecks {
				got = append(got, check.Name)
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("selected checks = %v, want %v; mode=%s fallback=%s", got, want, selection.Mode, selection.Fallback)
			}
			encoded, err := json.Marshal(selection)
			if err != nil {
				t.Fatal(err)
			}
			t.Log(string(encoded))
		})
	}
	selection := selectChecks("/repo", "github.com/acme/sdt", "", []Change{{Status: "M", Path: "internal/base/store.go"}}, fixturePackages(), nil, false)
	got := make([]string, 0, len(selection.SelectedChecks))
	for _, check := range selection.SelectedChecks {
		got = append(got, check.Name)
	}
	sort.Strings(got)
	want := append([]string(nil), all...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing-base selected checks = %v, want %v", got, want)
	}
	encoded, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("missing-base: %s", encoded)
}

func checkNames(specs []checkSpec) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.name)
	}
	return names
}

func fixturePackages() []Package {
	return []Package{
		{ImportPath: "github.com/acme/sdt", Dir: "/repo", Imports: []string{"github.com/acme/sdt/internal/base"}},
		{ImportPath: "github.com/acme/sdt/internal/base", Dir: "/repo/internal/base", Imports: []string{"fmt"}},
		{ImportPath: "github.com/acme/sdt/internal/middle", Dir: "/repo/internal/middle", Imports: []string{"github.com/acme/sdt/internal/base"}},
		{ImportPath: "github.com/acme/sdt/internal/acpbridge", Dir: "/repo/internal/acpbridge", Imports: []string{"github.com/acme/sdt/internal/base"}},
		{ImportPath: "github.com/acme/sdt/cli/cmd", Dir: "/repo/cli/cmd", Imports: []string{"github.com/acme/sdt/internal/middle"}},
		{ImportPath: "github.com/acme/sdt/viewer", Dir: "/repo/viewer", Imports: []string{"net/http"}},
	}
}

func TestGoChangeSelectsReverseDependencies(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "internal/base/store.go"}}, fixturePackages(), nil, false)
	if selection.Mode != "targeted" || selection.Fallback != "" {
		t.Fatalf("selection = %+v, want targeted", selection)
	}
	for _, name := range []string{"format", "build", "vet", "lint", "test", "gosec"} {
		check, ok := findCheck(selection.SelectedChecks, name)
		if !ok {
			t.Fatalf("selected checks missing %q: %+v", name, selection.SelectedChecks)
		}
		want := []string{".", "./cli/cmd", "./internal/acpbridge", "./internal/base", "./internal/middle"}
		if !reflect.DeepEqual(check.Packages, want) {
			t.Errorf("%s packages = %v, want %v", name, check.Packages, want)
		}
	}
	for _, name := range []string{"coverage", "trivy", "web"} {
		if _, ok := findCheck(selection.DeferredChecks, name); !ok {
			t.Errorf("%q should be explicitly deferred to CI: %+v", name, selection.DeferredChecks)
		}
	}
}

func TestWebChangeAddsViewerIntegration(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "web/src/App.tsx"}}, fixturePackages(), nil, false)
	if selection.Mode != "targeted" {
		t.Fatalf("mode = %q, want targeted", selection.Mode)
	}
	if _, ok := findCheck(selection.SelectedChecks, "web"); !ok {
		t.Fatalf("web suite not selected: %+v", selection.SelectedChecks)
	}
	for _, name := range []string{"build", "test"} {
		check, ok := findCheck(selection.SelectedChecks, name)
		if !ok || !reflect.DeepEqual(check.Packages, []string{"./viewer"}) {
			t.Errorf("%s = %+v, want viewer integration", name, check)
		}
	}
	if _, ok := findCheck(selection.SelectedChecks, "vet"); ok {
		t.Fatalf("web-only change selected unrelated Go vet: %+v", selection.SelectedChecks)
	}
}

func TestContextDocsSelectContextLint(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "context/plan/example.md"}}, fixturePackages(), nil, false)
	if selection.Mode != "targeted" || len(selection.SelectedChecks) != 1 || selection.SelectedChecks[0].Name != "context-lint" {
		t.Fatalf("selection = %+v, want context lint only", selection)
	}
}

func TestACPWireChangeSelectsGoAndWebIntegration(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "internal/acpbridge/testdata/wire/session.json"}}, fixturePackages(), nil, false)
	if selection.Mode != "targeted" {
		t.Fatalf("mode = %q, want targeted", selection.Mode)
	}
	if _, ok := findCheck(selection.SelectedChecks, "web"); !ok {
		t.Fatalf("ACP wire change did not select web checks: %+v", selection.SelectedChecks)
	}
	if check, ok := findCheck(selection.SelectedChecks, "test"); !ok || !contains(check.Packages, "./internal/acpbridge") {
		t.Fatalf("ACP wire change did not select its owning Go package: %+v", selection.SelectedChecks)
	}
}

func TestUncertainChangesFallBackToFull(t *testing.T) {
	cases := []Change{
		{Status: "M", Path: "Taskfile.yml"},
		{Status: "M", Path: "README.md"},
		{Status: "D", Path: "internal/removed/pkg.go"},
		{Status: "M", Path: "context/instructions/development.md"},
		{Status: "M", Path: "internal/templates/instructions/development.md.tmpl"},
		{Status: "M", Path: ".github/workflows/ci.yml"},
	}
	for _, change := range cases {
		selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{change}, fixturePackages(), nil, false)
		if selection.Mode != "full-fallback" || len(selection.SelectedChecks) != len(fullChecks) {
			t.Errorf("change %+v selected %q with %d checks, want full fallback", change, selection.Mode, len(selection.SelectedChecks))
		}
	}
}

func TestMissingBaseAndEmptyChangeSetFallBackToFull(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    string
		changes []Change
	}{
		{name: "missing base", changes: []Change{{Status: "M", Path: "internal/base/store.go"}}},
		{name: "no changes", base: "base-sha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection := selectChecks("/repo", "github.com/acme/sdt", tc.base, tc.changes, fixturePackages(), nil, false)
			if selection.Mode != "full-fallback" || selection.Fallback == "" {
				t.Fatalf("selection = %+v, want explicit full fallback", selection)
			}
		})
	}
}

func TestAgentPackagesOnlyAddChecks(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "internal/base/store.go"}}, fixturePackages(), []string{"github.com/acme/sdt/viewer"}, false)
	check, ok := findCheck(selection.SelectedChecks, "test")
	if !ok || !contains(check.Packages, "./viewer") {
		t.Fatalf("agent package did not add viewer to required Go tests: %+v", selection)
	}
	if !reflect.DeepEqual(selection.AgentPackages, []string{"github.com/acme/sdt/viewer"}) {
		t.Fatalf("agent additions = %v", selection.AgentPackages)
	}
}

func TestRenameCarriesBothPaths(t *testing.T) {
	selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "R100", Old: "internal/base/store.go", Path: "internal/middle/store.go"}}, fixturePackages(), nil, false)
	check, ok := findCheck(selection.SelectedChecks, "test")
	if !ok || !contains(check.Packages, "./internal/base") || !contains(check.Packages, "./internal/middle") {
		t.Fatalf("rename impact = %+v", selection)
	}
}

func TestParseNameStatusHandlesRenameAndDeletion(t *testing.T) {
	changes, err := parseNameStatus([]byte("M\x00internal/a.go\x00R100\x00old/name.go\x00new/name.go\x00D\x00gone/file.go\x00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Status: "M", Path: "internal/a.go"},
		{Status: "R100", Old: "old/name.go", Path: "new/name.go"},
		{Status: "D", Path: "gone/file.go"},
	}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("parsed changes = %#v, want %#v", changes, want)
	}
}

func TestFullCIGroupsCoverEveryConfiguredGateCheck(t *testing.T) {
	groups := []string{"test", "build", "lint", "web", "vuln"}
	covered := map[string]bool{}
	for _, group := range groups {
		checks := checksForGroup(group)
		if len(checks) == 0 {
			t.Fatalf("full CI group %q has no checks", group)
		}
		for _, check := range checks {
			covered[check.Name] = true
		}
	}
	for _, name := range []string{"format", "modules", "entry-metadata", "delivery-tool-test", "build", "vet", "lint", "test", "coverage", "web", "govulncheck"} {
		if !covered[name] {
			t.Errorf("existing CI check %q is missing from the full CI groups", name)
		}
	}
	for _, name := range []string{"gosec", "trivy"} {
		if covered[name] {
			t.Errorf("local-only gate check %q unexpectedly runs in the CI groups", name)
		}
	}
}

func TestCommandForUsesScopedPackagesLocallyAndSharedScriptsInFullMode(t *testing.T) {
	local, err := commandFor(Check{Name: "test", Packages: []string{"./internal/base"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(local, []string{"go", "test", "./internal/base"}) {
		t.Fatalf("local test command = %v", local)
	}
	full, err := commandFor(Check{Name: "test"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full, []string{"sh", "scripts/verify.sh", "test"}) {
		t.Fatalf("full test command = %v", full)
	}
	if _, err := commandFor(Check{Name: "test"}, false); err == nil {
		t.Fatal("targeted test without package scope must fail")
	}
}

func TestMergeChecksUnifiesScopesAndReasons(t *testing.T) {
	merged := mergeChecks([]Check{
		{Name: "test", Status: "selected", Reason: "Go change", Packages: []string{"./cli/cmd", "./internal/base"}},
		{Name: "test", Status: "selected", Reason: "viewer embed", Packages: []string{"./viewer", "./internal/base"}},
	})
	if len(merged) != 1 {
		t.Fatalf("merged checks = %+v", merged)
	}
	wantPackages := []string{"./cli/cmd", "./internal/base", "./viewer"}
	if !reflect.DeepEqual(merged[0].Packages, wantPackages) {
		t.Errorf("merged packages = %v, want %v", merged[0].Packages, wantPackages)
	}
	if merged[0].Reason != "Go change; viewer embed" {
		t.Errorf("merged reason = %q", merged[0].Reason)
	}
}

func findCheck(checks []Check, name string) (Check, bool) {
	for _, check := range checks {
		if check.Name == name {
			return check, true
		}
	}
	return Check{}, false
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func TestAddedPackageAliasesResolveToExecutableGoScopes(t *testing.T) {
	packages := fixturePackages()
	for _, alias := range []string{".", "./", "./internal/base", "internal/base", "github.com/acme/sdt/internal/base"} {
		t.Run(alias, func(t *testing.T) {
			selection := selectChecks("/repo", "github.com/acme/sdt", "base-sha", []Change{{Status: "M", Path: "internal/middle/file.go"}}, packages, []string{alias}, false)
			if selection.Mode != "targeted" {
				t.Fatalf("selection mode = %s", selection.Mode)
			}
			check, ok := findCheck(selection.SelectedChecks, "test")
			if !ok {
				t.Fatalf("test check absent: %+v", selection.SelectedChecks)
			}
			want := alias
			if alias == "." || alias == "./" {
				want = "."
			}
			if alias == "internal/base" || alias == "github.com/acme/sdt/internal/base" {
				want = "./internal/base"
			}
			if !contains(check.Packages, want) {
				t.Errorf("test scope %v missing %s", check.Packages, want)
			}
		})
	}
}
