package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStringListSet(t *testing.T) {
	var values stringList
	if err := values.Set("./internal/base"); err != nil {
		t.Fatal(err)
	}
	if err := values.Set(" "); err == nil {
		t.Fatal("empty package was accepted")
	}
	if values.String() != "./internal/base" {
		t.Fatalf("values = %q", values.String())
	}
}

func TestCommandForRecognizesFullChecksAndTargetedCommands(t *testing.T) {
	for _, check := range fullChecks {
		if _, err := commandFor(Check{Name: check.name}, true); err != nil {
			t.Errorf("full command for %s: %v", check.name, err)
		}
	}
	for _, check := range []Check{
		{Name: "context-lint"},
		{Name: "web"},
		{Name: "entry-metadata"},
		{Name: "format", Packages: []string{"."}},
		{Name: "build", Packages: []string{"./internal/base"}},
		{Name: "vet", Packages: []string{"./internal/base"}},
		{Name: "lint", Packages: []string{"./internal/base"}},
		{Name: "test", Packages: []string{"./internal/base"}},
		{Name: "gosec", Packages: []string{"./internal/base"}},
	} {
		if _, err := commandFor(check, false); err != nil {
			t.Errorf("targeted command for %s: %v", check.Name, err)
		}
	}
	for _, check := range []Check{{Name: "unknown", Packages: []string{"./pkg"}}, {Name: "test"}} {
		if _, err := commandFor(check, false); err == nil {
			t.Errorf("invalid targeted command accepted: %+v", check)
		}
	}
	if _, err := commandFor(Check{Name: "unknown"}, true); err == nil {
		t.Fatal("unknown full check accepted")
	}
}

func TestExecuteCheckReportsPassAndFormatFailure(t *testing.T) {
	root := newGitFixture(t)
	writeFixture(t, root, "go.mod", "module example.test/project\n\ngo 1.27.4\n")
	writeFixture(t, root, "pkg/value.go", "package pkg\nfunc Value( ) int { return 1 }\n")
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	failed := executeCheck(root, Check{Name: "format", Packages: []string{"./pkg"}}, false)
	if failed.Status != "failed" || failed.ExitCode != 1 || !strings.Contains(failed.Error, "unformatted Go files") || failed.ElapsedMS < 0 || failed.FailureOutput == "" {
		t.Fatalf("format result = %+v", failed)
	}
	if data, err := os.ReadFile(filepath.Join(root, failed.FailureOutput)); err != nil || !strings.Contains(string(data), "value.go") {
		t.Fatalf("failure output reference %q: data=%q err=%v", failed.FailureOutput, data, err)
	}
	writeFixture(t, root, "pkg/value.go", "package pkg\n\nfunc Value() int { return 1 }\n")
	passed := executeCheck(root, Check{Name: "test", Packages: []string{"./pkg"}}, false)
	if passed.Status != "passed" || passed.ExitCode != 0 || passed.Error != "" || passed.ElapsedMS < 0 {
		t.Fatalf("test result = %+v", passed)
	}
}

func TestFinishResultClassifiesExecutionErrors(t *testing.T) {
	started := time.Now()
	result := Result{Name: "test"}
	finishResult(&result, started, errors.New("missing command"))
	if result.Status != "failed" || result.ExitCode != 1 || result.FailureOutput != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestOutputFormattersExplainPreviewAndResults(t *testing.T) {
	var preview bytes.Buffer
	writePreviewText(&preview, gateReport{Outcome: "preview", Plan: Selection{
		Mode: "targeted", Base: "base-sha", ChangedFiles: []Change{{Status: "R100", Old: "old.go", Path: "new.go"}},
		SelectedChecks:   []Check{{Name: "test", Reason: "changed Go package", Packages: []string{"./pkg"}}},
		DeferredChecks:   []Check{{Name: "coverage", Reason: "CI runs complete coverage"}},
		OutOfScopeChecks: []Check{{Name: "context-lint", Reason: "no corpus change"}}, AgentPackages: []string{"./extra"},
	}})
	for _, expected := range []string{"mode: targeted", "old.go -> new.go", "selected: test", "deferred: coverage", "out of scope: context-lint", "agent-added packages", "selection preview only; no checks run"} {
		if !strings.Contains(preview.String(), expected) {
			t.Errorf("preview output missing %q", expected)
		}
	}

	var results bytes.Buffer
	writeResultText(&results, gateReport{Outcome: "failed", Results: []Result{{Name: "test", Status: "failed", ElapsedMS: 42, ExitCode: 1, Error: "exit status 1"}}})
	if !strings.Contains(results.String(), "test: failed (42 ms) — exit status 1") || !strings.Contains(results.String(), "FAILED") {
		t.Fatalf("result output = %q", results.String())
	}
}

func TestValidateAddedPackagesAcceptsProductPathsAndRejectsUnknown(t *testing.T) {
	packages := fixturePackages()
	if err := validateAddedPackages([]string{".", "./internal/base", "github.com/acme/sdt/viewer"}, packages); err != nil {
		t.Fatalf("valid packages rejected: %v", err)
	}
	if err := validateAddedPackages([]string{"./not-a-package"}, packages); err == nil {
		t.Fatal("unknown package accepted")
	}
}

func TestValueOrUsesFallback(t *testing.T) {
	if got := valueOr("", "unavailable"); got != "unavailable" {
		t.Fatalf("empty value = %q", got)
	}
	if got := valueOr("base", "unavailable"); got != "base" {
		t.Fatalf("set value = %q", got)
	}
}
