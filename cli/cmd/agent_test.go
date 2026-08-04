package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runInTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ── section helpers ────────────────────────────────────────────────────────────

func TestSectionBlockHelpers(t *testing.T) {
	content := "# Header\n\n" + sectionBlock("a", "bodyA")

	out, err := addSectionBlock(content, "b", "bodyB")
	if err != nil {
		t.Fatal(err)
	}
	if !hasSection(out, "a") || !hasSection(out, "b") {
		t.Error("expected both sections present after add")
	}
	if _, err := addSectionBlock(out, "b", "dup"); err == nil {
		t.Error("expected error adding duplicate section")
	}

	out, err = updateSectionBlock(out, "a", "newA")
	if err != nil {
		t.Fatal(err)
	}
	body, ok := getSectionBody(out, "a")
	if !ok || body != "newA" {
		t.Errorf("expected updated body newA, got %q (found=%v)", body, ok)
	}
	if _, err := updateSectionBlock(out, "missing", "x"); err == nil {
		t.Error("expected error updating missing section")
	}

	names := listSections(out)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("unexpected section list: %v", names)
	}

	out, err = removeSectionBlock(out, "a")
	if err != nil {
		t.Fatal(err)
	}
	if hasSection(out, "a") {
		t.Error("expected section a removed")
	}
	if !hasSection(out, "b") {
		t.Error("expected section b still present")
	}
}

func TestValidateSectionName(t *testing.T) {
	for _, good := range []string{"a", "memory", "self-update", "build2"} {
		if err := validateSectionName(good); err != nil {
			t.Errorf("expected %q valid, got error: %v", good, err)
		}
	}
	for _, bad := range []string{"", "Bad Name", "UPPER", "a b", "-x", "a/b"} {
		if err := validateSectionName(bad); err == nil {
			t.Errorf("expected %q invalid", bad)
		}
	}
}

func TestSectionBlockMissingPaths(t *testing.T) {
	if _, err := removeSectionBlock("# H", "missing"); err == nil {
		t.Error("expected error removing missing section")
	}
	if _, ok := getSectionBody("# H", "missing"); ok {
		t.Error("expected ok=false for missing section body")
	}
}

func TestAgentTargetPathFallback(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "AGENTS.md", sectionBlock("a", "x"))
	out := execute(t, agentSectionListCmd, nil, "--target", "")
	if !strings.Contains(string(out), "a") {
		t.Errorf("expected fallback to AGENTS.md, got: %s", out)
	}
}

func TestAgentReadTargetDirError(t *testing.T) {
	runInTempDir(t)
	if err := os.Mkdir("adir", 0o750); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentSectionListCmd, nil, "--target", "adir"))
	})
}

func TestAgentSectionExtraArgs(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "# H\n")
	execute(t, agentSectionAddCmd, nil, "build", "run", "make", "all")
	data, _ := os.ReadFile("AGENTS.md")
	if !strings.Contains(string(data), "run make all") {
		t.Errorf("expected extra args joined as content:\n%s", data)
	}
}

// ── agent section commands ─────────────────────────────────────────────────────

func TestAgentSectionAddUpdateRemove(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "# My Project\n")

	out := execute(t, agentSectionAddCmd, []byte("run go test"), "tools")
	if !strings.Contains(string(out), "added section") {
		t.Errorf("unexpected add output: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "run go test") {
		t.Error("expected section content in AGENTS.md")
	}

	out = execute(t, agentSectionListCmd, nil)
	if !strings.Contains(string(out), "tools") {
		t.Errorf("expected 'tools' in list output: %s", out)
	}

	out = execute(t, agentSectionShowCmd, nil, "tools")
	if !strings.Contains(string(out), "run go test") {
		t.Errorf("expected section body in show output: %s", out)
	}

	out = execute(t, agentSectionUpdateCmd, []byte("run go vet"), "tools")
	if !strings.Contains(string(out), "updated section") {
		t.Errorf("unexpected update output: %s", out)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "run go vet") {
		t.Error("expected updated content in AGENTS.md")
	}

	out = execute(t, agentSectionSetCmd, []byte("run go build"), "tools")
	if !strings.Contains(string(out), "updated section") {
		t.Errorf("expected set to update: %s", out)
	}
	out = execute(t, agentSectionSetCmd, []byte("extra"), "newone")
	if !strings.Contains(string(out), "added section") {
		t.Errorf("expected set to add: %s", out)
	}

	out = execute(t, agentSectionRemoveCmd, nil, "tools")
	if !strings.Contains(string(out), "removed section") {
		t.Errorf("unexpected remove output: %s", out)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(data), "run go build") {
		t.Error("expected section removed from AGENTS.md")
	}
}

func TestAgentSectionMissingTarget(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentSectionListCmd, nil))
	})
}

func TestAgentSectionDuplicate(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "AGENTS.md", sectionBlock("a", "x"))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentSectionAddCmd, []byte("y"), "a"))
	})
}

func TestAgentSectionEmptyContent(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "# H\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentSectionAddCmd, []byte("   "), "empty"))
	})
}

// ── agent init ─────────────────────────────────────────────────────────────────

func TestAgentInit(t *testing.T) {
	dir := runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "myapp", "--group", "platform")
	if !strings.Contains(string(out), "AGENTS.md") {
		t.Errorf("unexpected init output: %s", out)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	for _, s := range []string{"project", "commands", "workflow", "memory", "planning", "annotations", "self-update"} {
		if !strings.Contains(string(data), "<!-- sdt:begin:"+s+" -->") {
			t.Errorf("expected section %q in generated AGENTS.md", s)
		}
	}
	for _, want := range []string{"sdt memory", "sdt.context/worklog/", "frontmatter", "sdt agent section update", "sdt.context/plan/", "sdt.context/tmp/"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in opinionated AGENTS.md", want)
		}
	}

	cfg, _ := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	for _, want := range []string{"project: myapp", "group: platform"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("expected %q in .sdt.yaml", want)
		}
	}
	if strings.Contains(string(cfg), "project_id") || strings.Contains(string(cfg), "group_id") {
		t.Errorf("did not expect id fields in .sdt.yaml:\n%s", cfg)
	}
}

func TestAgentInitNoProject(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil)
	data, err := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	if err != nil {
		t.Fatalf("expected .sdt.yaml created: %v", err)
	}
	if !strings.Contains(string(data), "project:") {
		t.Errorf("expected default project in .sdt.yaml:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Error("expected AGENTS.md created without --project")
	}
	for _, d := range []string{"sdt.context/plan", "sdt.context/worklog", "sdt.context/notes", "sdt.context/tmp"} {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("expected %s to be created", d)
		}
	}
}

func TestAgentInitExisting(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "existing")
	execute(t, agentInitCmd, nil, "--project", "p")
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "existing") {
		t.Error("expected custom content preserved (non-destructive)")
	}
	for _, s := range []string{"project", "commands", "workflow", "memory", "planning", "annotations", "self-update"} {
		if !strings.Contains(string(data), "<!-- sdt:begin:"+s+" -->") {
			t.Errorf("expected section %q added to existing AGENTS.md", s)
		}
	}
}

func TestAgentInitForce(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "existing")
	execute(t, agentInitCmd, nil, "--project", "p", "--force")
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "existing") {
		t.Error("expected leading custom content preserved with --force")
	}
	if !strings.Contains(string(data), "- Project: p") {
		t.Error("expected project section refreshed with --force")
	}
}

func TestAgentInitEmptyTarget(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "")
	execute(t, agentInitCmd, nil, "--project", "p")
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	for _, s := range []string{"project", "commands", "workflow", "memory", "planning", "annotations", "self-update"} {
		if !strings.Contains(string(data), "<!-- sdt:begin:"+s+" -->") {
			t.Errorf("expected section %q in generated AGENTS.md", s)
		}
	}
}

func TestAgentInitIdempotent(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "myapp", "--group", "platform")
	first, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	firstCfg, _ := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	out := execute(t, agentInitCmd, nil, "--project", "myapp", "--group", "platform")
	if !strings.Contains(string(out), "skipped") {
		t.Errorf("expected second run to skip unchanged files: %s", out)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(first) != string(second) {
		t.Error("expected AGENTS.md unchanged on second run")
	}
	secondCfg, _ := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	if string(firstCfg) != string(secondCfg) {
		t.Errorf("expected .sdt.yaml unchanged on second run\n--- first ---\n%s\n--- second ---\n%s", firstCfg, secondCfg)
	}
}

func TestAgentInitPreservesConfig(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: existing\n")
	execute(t, agentInitCmd, nil, "--project", "new", "--group", "g", "--yes")
	data, _ := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	for _, want := range []string{"project: existing", "group: g"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q preserved in .sdt.yaml:\n%s", want, data)
		}
	}
}

func TestAgentInitJSONOutput(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--format", "json")
	var res []FileResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON from init: %v\n%s", err, out)
	}
	if len(res) < 8 {
		t.Errorf("expected config+dirs+md+guide results, got %d", len(res))
	}
}

// ── agent guide ────────────────────────────────────────────────────────────────

func TestAgentGuide(t *testing.T) {
	dir := runInTempDir(t)
	out := execute(t, agentGuideCmd, nil)
	if !strings.Contains(string(out), "[created]") {
		t.Errorf("unexpected guide output: %s", out)
	}
	for _, f := range []string{"SKILL.md", "REFERENCE.md", "WORKFLOWS.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".agents/skills/sdt", f)); err != nil {
			t.Errorf("expected %s to be created", f)
		}
	}
	skill, _ := os.ReadFile(filepath.Join(dir, ".agents/skills/sdt", "SKILL.md"))
	if !strings.Contains(string(skill), "name: sdt") {
		t.Error("SKILL.md should contain frontmatter with name: sdt")
	}

	out = execute(t, agentGuideCmd, nil)
	if !strings.Contains(string(out), "skipped") {
		t.Errorf("expected skipped on second run: %s", out)
	}
}

func TestAgentGuideForce(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentGuideCmd, nil)
	writeTestFile(t, filepath.Join(".agents", "skills", "sdt", "SKILL.md"), "custom")
	execute(t, agentGuideCmd, nil, "--force")
	skill, _ := os.ReadFile(filepath.Join(dir, ".agents", "skills", "sdt", "SKILL.md"))
	if strings.Contains(string(skill), "custom") {
		t.Error("expected SKILL.md overwritten with --force")
	}
}

func TestAgentGuideJSONOutput(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentGuideCmd, nil, "--format", "json")
	var res []FileResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON from guide: %v\n%s", err, out)
	}
	if len(res) != 3 {
		t.Errorf("expected 3 guide files, got %d", len(res))
	}
}

func TestAgentGuideDryRun(t *testing.T) {
	dir := runInTempDir(t)
	out := execute(t, agentGuideCmd, nil, "--dry-run")
	if !strings.Contains(string(out), "dry-run") {
		t.Errorf("expected dry-run in output: %s", out)
	}
	for _, f := range []string{"SKILL.md", "REFERENCE.md", "WORKFLOWS.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".agents/skills/sdt", f)); !os.IsNotExist(err) {
			t.Errorf("dry-run: %s should not exist", f)
		}
	}
}

func TestAgentGuideYAMLOutput(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentGuideCmd, nil, "--format", "yaml")
	if !strings.Contains(string(out), "path:") {
		t.Errorf("expected yaml list output: %s", out)
	}
}

func TestAgentGuideDirError(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, filepath.Join(".agents", "skills", "sdt"), "file in the way")
	out := execute(t, agentGuideCmd, nil)
	if !strings.Contains(string(out), "[error]") {
		t.Errorf("expected error status for blocked guide dir: %s", out)
	}
}

func TestAgentInitWorkDirError(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "sdt.context", "file in the way")
	out := execute(t, agentInitCmd, nil, "--project", "p")
	if !strings.Contains(string(out), "[error]") {
		t.Errorf("expected error status for blocked sdt.context/ dir: %s", out)
	}
}

func TestAgentInitYAMLOutput(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--format", "yaml")
	if !strings.Contains(string(out), "path:") {
		t.Errorf("expected yaml list output: %s", out)
	}
}

func TestLoadProjectConfigInvalid(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: [unclosed\n")
	if cfg := loadProjectConfig(".sdt.yaml"); cfg != nil {
		t.Error("expected nil for invalid yaml")
	}
	if cfg := loadProjectConfig("missing.yaml"); cfg != nil {
		t.Error("expected nil for missing file")
	}
}

func TestAgentPromptNonTTY(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if !strings.Contains(string(out), "[written]") {
		t.Errorf("expected init output: %s", out)
	}
}

// ── config ─────────────────────────────────────────────────────────────────────

func TestBuildProjectConfigContent(t *testing.T) {
	content := buildProjectConfigContent("myapp", "grp")
	for _, want := range []string{"project: myapp", "group: grp"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in config content, got:\n%s", want, content)
		}
	}
}

func TestConfigInit(t *testing.T) {
	dir := runInTempDir(t)
	out := execute(t, configInitCmd, nil, "--project", "myapp", "--group", "grp")
	if !strings.Contains(string(out), ".sdt.yaml") {
		t.Errorf("unexpected config init output: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	for _, want := range []string{"project: myapp", "group: grp"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in .sdt.yaml:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "project_id") || strings.Contains(string(data), "group_id") {
		t.Errorf("did not expect id fields in .sdt.yaml:\n%s", data)
	}
}

func TestConfigInitExisting(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: old\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, configInitCmd, nil, "--project", "p"))
	})
}

func TestConfigInitMissingProject(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, configInitCmd, nil))
	})
}

func TestConfigShow(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: myapp\ngroup: grp\n")
	out := execute(t, configShowCmd, nil, "--format", "json")
	var cfg ProjectConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("invalid JSON from config show: %v\n%s", err, out)
	}
	if cfg.Project != "myapp" || cfg.Group != "grp" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestConfigShowNotFound(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, configShowCmd, nil))
	})
}

func TestDefaultProjectName(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Base(cwd) + "_"
	if got := defaultProjectName(); !strings.HasPrefix(got, want) {
		t.Errorf("expected default to start with %q, got %q", want, got)
	}
	if len(defaultProjectName()) <= len(filepath.Base(cwd)) {
		t.Error("expected default to include a short path hash")
	}
}

func TestProjectConfigFillNil(t *testing.T) {
	cfg := &ProjectConfig{Project: "p", Group: "g"}
	cfg.fill(nil)
	if cfg.Project != "p" || cfg.Group != "g" {
		t.Error("fill(nil) should preserve existing values")
	}
}
