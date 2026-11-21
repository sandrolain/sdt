package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryIncludesStagedUnstagedUntrackedRenamesAndDeletes(t *testing.T) {
	root := newGitFixture(t)
	writeFixture(t, root, "pkg/staged.go", "package pkg\n\nconst Staged = 1\n")
	writeFixture(t, root, "pkg/unstaged.go", "package pkg\n\nconst Unstaged = 1\n")
	writeFixture(t, root, "pkg/rename.go", "package pkg\n\nconst RenameMe = 1\n")
	writeFixture(t, root, "pkg/deleted.go", "package pkg\n\nconst DeleteMe = 1\n")
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "commit", "-m", "base")

	writeFixture(t, root, "pkg/staged.go", "package pkg\n\nconst Staged = 2\n")
	gitFixture(t, root, "add", "pkg/staged.go")
	writeFixture(t, root, "pkg/unstaged.go", "package pkg\n\nconst Unstaged = 2\n")
	if err := os.Rename(filepath.Join(root, "pkg/rename.go"), filepath.Join(root, "pkg/renamed.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "pkg/deleted.go")); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, "add", "-A", "pkg/rename.go", "pkg/renamed.go", "pkg/deleted.go")
	writeFixture(t, root, "pkg/untracked.go", "package pkg\n\nconst Untracked = 1\n")

	changes, base, fallback := inventoryChanges(root, "main")
	if base == "" || fallback != "" {
		t.Fatalf("inventory base=%q fallback=%q", base, fallback)
	}
	assertChange(t, changes, "M", "pkg/staged.go", "")
	assertChange(t, changes, "M", "pkg/unstaged.go", "")
	assertChange(t, changes, "R100", "pkg/renamed.go", "pkg/rename.go")
	assertChange(t, changes, "D", "pkg/deleted.go", "")
	assertChange(t, changes, "?", "pkg/untracked.go", "")
}

func TestRunAtPreviewsTargetedGoSelectionWithoutReportingPass(t *testing.T) {
	root := newGitFixture(t)
	writeFixture(t, root, "go.mod", "module example.test/project\n\ngo 1.27.4\n")
	writeFixture(t, root, "pkg/value.go", "package pkg\n\nfunc Value() int { return 1 }\n")
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "commit", "-m", "base")
	writeFixture(t, root, "pkg/value.go", "package pkg\n\nfunc Value() int { return 2 }\n")

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var stdout, stderr bytes.Buffer
	if err := runAt([]string{"--preview", "--base", "HEAD", "--format", "json"}, &stdout, &stderr, root); err != nil {
		t.Fatalf("run preview: %v; stderr=%s", err, stderr.String())
	}
	var report gateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode preview: %v\n%s", err, stdout.String())
	}
	if report.Outcome != "preview" || report.Passed != nil || len(report.Results) != 0 {
		t.Fatalf("preview reported execution: %+v", report)
	}
	if report.Plan.Mode != "targeted" || report.Plan.Base == "" {
		t.Fatalf("selection = %+v", report.Plan)
	}
	testCheck, ok := findCheck(report.Plan.SelectedChecks, "test")
	if !ok || !strings.Contains(strings.Join(testCheck.Packages, ","), "./pkg") {
		t.Fatalf("targeted Go test scope = %+v", testCheck)
	}
}

func TestRunAtFallsBackWhenComparisonBaseIsMissing(t *testing.T) {
	root := newGitFixture(t)
	writeFixture(t, root, "go.mod", "module example.test/project\n\ngo 1.27.4\n")
	writeFixture(t, root, "pkg/value.go", "package pkg\n")
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "commit", "-m", "base")

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var stdout, stderr bytes.Buffer
	if err := runAt([]string{"--preview", "--base", "missing-ref", "--format", "json"}, &stdout, &stderr, root); err != nil {
		t.Fatalf("run fallback preview: %v; stderr=%s", err, stderr.String())
	}
	var report gateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Plan.Mode != "full-fallback" || !strings.Contains(report.Plan.Fallback, "merge base") {
		t.Fatalf("selection = %+v, want full fallback", report.Plan)
	}
}

func TestRunAtLabelsCIGroupAsGroupRatherThanFullSuite(t *testing.T) {
	root := newGitFixture(t)
	writeFixture(t, root, "go.mod", "module example.test/project\n\ngo 1.27.4\n")
	writeFixture(t, root, "pkg/value.go", "package pkg\n")
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "commit", "-m", "base")

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var stdout, stderr bytes.Buffer
	if err := runAt([]string{"--full", "--group", "test", "--preview", "--format", "json"}, &stdout, &stderr, root); err != nil {
		t.Fatalf("run group preview: %v; stderr=%s", err, stderr.String())
	}
	var report gateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Plan.Mode != "full-group" || report.Plan.Group != "test" || report.Outcome != "preview" || report.Passed != nil {
		t.Fatalf("group preview report = %+v", report)
	}
}

func newGitFixture(t *testing.T) string {
	t.Helper()
	tmpRoot := filepath.Join("..", "..", "context", "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(tmpRoot, "verify-delivery-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	gitFixture(t, root, "init", "-b", "main")
	gitFixture(t, root, "config", "user.email", "verify-test@example.invalid")
	gitFixture(t, root, "config", "user.name", "Verification Test")
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func writeFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitFixture(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func assertChange(t *testing.T, changes []Change, status, path, old string) {
	t.Helper()
	for _, change := range changes {
		if change.Status == status && change.Path == path && change.Old == old {
			return
		}
	}
	t.Fatalf("missing change status=%s old=%q path=%q in %+v", status, old, path, changes)
}

func TestParsePackageInventoryRejectsIncompleteProductGraph(t *testing.T) {
	cases := []struct{ name, record string }{
		{"incomplete", `{"ImportPath":"example.test/p/pkg","Dir":"/repo/pkg","Incomplete":true}`},
		{"package error", `{"ImportPath":"example.test/p/pkg","Dir":"/repo/pkg","Error":{"Err":"bad import"}}`},
		{"dependency error", `{"ImportPath":"example.test/p/pkg","Dir":"/repo/pkg","DepsErrors":[{"Err":"bad dependency"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parsePackageInventory([]byte(tc.record), "example.test/p"); err == nil {
				t.Fatal("incomplete inventory accepted")
			}
		})
	}
}

func TestParsePackageInventorySkipsExcludedContextBeforeIncompleteCheck(t *testing.T) {
	data := []byte(`{"ImportPath":"example.test/p/context/broken","Dir":"/repo/context/broken","Incomplete":true}
{"ImportPath":"example.test/p/cli","Dir":"/repo/cli"}`)
	packages, err := parsePackageInventory(data, "example.test/p")
	if err != nil || len(packages) != 1 || packages[0].ImportPath != "example.test/p/cli" {
		t.Fatalf("packages=%+v err=%v", packages, err)
	}
}
