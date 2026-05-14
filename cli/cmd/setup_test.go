package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to run setup command in a temp dir
func runSetupInTempDir(t *testing.T, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})

	out := execute(t, setupCmd, nil, args...)
	return strings.TrimSpace(string(out)), nil
}

func TestSetupAllAgents(t *testing.T) {
	_, err := runSetupInTempDir(t, "--project", "testproject")
	if err != nil {
		t.Fatal(err)
	}

	dir, _ := os.Getwd()
	// --agent all (default) creates only AGENTS.md and skill file
	for _, path := range []string{
		".sdt.yaml",
		"AGENTS.md",
		".agents/skills/sdt/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created", path)
		}
	}
	// copilot and claude are NOT created by default
	for _, path := range []string{
		".github/copilot-instructions.md",
		"CLAUDE.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Errorf("expected %s NOT to be created by default", path)
		}
	}
}

func TestSetupSDTYAMLContent(t *testing.T) {
	_, err := runSetupInTempDir(t, "--project", "myapp", "--group", "platform")
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := os.Getwd()
	data, readErr := os.ReadFile(filepath.Join(dir, ".sdt.yaml")) //#nosec G304 -- test temp directory path
	if readErr != nil {
		t.Fatal(readErr)
	}
	content := string(data)
	if !strings.Contains(content, "project: myapp") {
		t.Errorf("expected project: myapp in .sdt.yaml, got: %s", content)
	}
	if !strings.Contains(content, "group: platform") {
		t.Errorf("expected group: platform in .sdt.yaml, got: %s", content)
	}
}

func TestSetupNoProject(t *testing.T) {
	_, err := runSetupInTempDir(t, "--agent", "generic")
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(dir, ".sdt.yaml")); !os.IsNotExist(err) {
		t.Error("expected .sdt.yaml NOT to be created when no --project given")
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); os.IsNotExist(err) {
		t.Error("expected AGENTS.md to be created")
	}
}

func TestSetupSpecificAgent(t *testing.T) {
	_, err := runSetupInTempDir(t, "--project", "myapp", "--agent", "copilot")
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(dir, ".github/copilot-instructions.md")); os.IsNotExist(err) {
		t.Error("expected copilot-instructions.md to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("expected CLAUDE.md NOT to be created for --agent copilot")
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("expected AGENTS.md NOT to be created for --agent copilot")
	}
}

func TestSetupSkipExisting(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// pre-create AGENTS.md with custom content
	_ = os.WriteFile("AGENTS.md", []byte("original"), 0o600)

	out := string(execute(t, setupCmd, nil, "--agent", "generic"))
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected skipped in output, got: %s", out)
	}
	data, _ := os.ReadFile("AGENTS.md")
	if string(data) != "original" {
		t.Error("existing AGENTS.md should not be overwritten without --force")
	}
}

func TestSetupForce(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// pre-create file
	_ = os.WriteFile("AGENTS.md", []byte("original"), 0o600)

	out := string(execute(t, setupCmd, nil, "--agent", "generic", "--force"))
	if strings.Contains(out, "skipped") {
		t.Errorf("expected file to be created with --force, got: %s", out)
	}
	data, _ := os.ReadFile("AGENTS.md")
	if string(data) == "original" {
		t.Error("--force should overwrite AGENTS.md")
	}
}

func TestSetupDryRun(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out := string(execute(t, setupCmd, nil, "--project", "myapp", "--dry-run"))
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run in output, got: %s", out)
	}
	// no files should exist
	for _, path := range []string{".sdt.yaml", "AGENTS.md", ".agents/skills/sdt/SKILL.md", "CLAUDE.md", ".github/copilot-instructions.md"} {
		if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Errorf("dry-run: file %s should not have been created", path)
		}
	}
}

func TestSetupJSONOutput(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out := execute(t, setupCmd, nil, "--project", "myapp", "--agent", "generic", "--format", "json")
	var result SetupResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if result.Project != "myapp" {
		t.Errorf("expected project=myapp, got %q", result.Project)
	}
	if len(result.Files) == 0 {
		t.Error("expected at least one file in JSON output")
	}
}

func TestSetupSkillAgent(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out := string(execute(t, setupCmd, nil, "--agent", "skill"))
	_ = out

	skillPath := filepath.Join(dir, ".agents", "skills", "sdt", "SKILL.md")
	data, err := os.ReadFile(skillPath) //#nosec G304 -- test temp directory path
	if err != nil {
		t.Fatalf("expected .agents/skills/sdt/SKILL.md to be created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: sdt") {
		t.Errorf("SKILL.md should contain YAML frontmatter with name: sdt\ngot: %s", content)
	}
	if !strings.Contains(content, "---") {
		t.Errorf("SKILL.md should contain YAML frontmatter delimiters")
	}
}

func TestSetupAllIncludesSkill(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	execute(t, setupCmd, nil, "--project", "myapp")

	for _, path := range []string{
		".sdt.yaml",
		"AGENTS.md",
		".agents/skills/sdt/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); os.IsNotExist(err) {
			t.Errorf("expected %s to be created by --agent all", path)
		}
	}
}

func TestSetupUnknownAgent(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		dir := t.TempDir()
		orig, _ := os.Getwd()
		_ = os.Chdir(dir)
		t.Cleanup(func() { _ = os.Chdir(orig) })
		return string(execute(t, setupCmd, nil, "--agent", "unknown"))
	})
}
