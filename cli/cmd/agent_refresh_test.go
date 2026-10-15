package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refreshProject prepares a temporary project and renders the generated set
// once, so each test starts from a project `sdt agent refresh` can act on.
func refreshProject(t *testing.T) *ProjectConfig {
	t.Helper()
	runInTempDir(t)
	writeTestFile(t, sdtConfigFile, "project: p\ngroup: g\n")
	cfg := &ProjectConfig{Project: "p", Group: "g"}
	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("initial render: %v", err)
	}
	return cfg
}

func TestAgentRefreshRestoresDriftedInstructionFile(t *testing.T) {
	cfg := refreshProject(t)
	path := filepath.Join(sdtInstrDir, "plan.md")
	writeTestFile(t, path, "<!-- sdt:begin:instructions/plan -->\n# STALE\n<!-- sdt:end:instructions/plan -->\n")

	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "# STALE") {
		t.Error("expected the stale generated body to be replaced by refresh")
	}
	if !strings.Contains(string(data), "# Plan Documents") {
		t.Error("expected the template render present after refresh")
	}
}

func TestAgentRefreshPreservesProjectBlock(t *testing.T) {
	cfg := refreshProject(t)
	_, body := agentMergeTarget(agentTargetDefault, "p", "g", true, true)
	writeTestFile(t, agentTargetDefault, body)

	data, err := os.ReadFile(agentTargetDefault)
	if err != nil {
		t.Fatal(err)
	}
	custom := strings.Replace(string(data), "<!-- sdt:end:project -->",
		"CUSTOM PROJECT RULE\n\n<!-- sdt:end:project -->", 1)
	if custom == string(data) {
		t.Fatal("test setup: project block end marker not found")
	}
	writeTestFile(t, agentTargetDefault, custom)

	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	after, err := os.ReadFile(agentTargetDefault)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "CUSTOM PROJECT RULE") {
		t.Error("refresh rewrote the write-once project block")
	}
}

func TestAgentRefreshLeavesSeedFilesCreateOnly(t *testing.T) {
	cfg := refreshProject(t)
	const topics = "topics:\n  - custom-topic\n"
	const readme = "# my own readme\n"
	writeTestFile(t, filepath.Join("context", "topics.yaml"), topics)
	writeTestFile(t, filepath.Join("context", readmeFile), readme)

	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join("context", "topics.yaml"): topics,
		filepath.Join("context", readmeFile):    readme,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s was rewritten by refresh:\n%s", path, got)
		}
	}
}

func TestAgentRefreshLeavesConfigAndGitignore(t *testing.T) {
	cfg := refreshProject(t)
	const config = "project: custom\ngroup: custom\n"
	const gitignore = "custom-ignore/\n"
	writeTestFile(t, sdtConfigFile, config)
	writeTestFile(t, ".gitignore", gitignore)

	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for path, want := range map[string]string{
		sdtConfigFile: config,
		".gitignore":  gitignore,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s was rewritten by refresh:\n%s", path, got)
		}
	}
}

func TestAgentRefreshArchivesObsoleteInstructionFile(t *testing.T) {
	cfg := refreshProject(t)
	obsolete := filepath.Join(sdtInstrDir, readmeFile)
	writeTestFile(t, obsolete, "# legacy instruction file\n")

	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := os.Stat(obsolete); !os.IsNotExist(err) {
		t.Errorf("expected the obsolete file moved out of %s", sdtInstrDir)
	}
	matches, err := filepath.Glob(filepath.Join(sdtTrashDir, "README-DEPRECATED-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Errorf("expected an archived copy under %s", sdtTrashDir)
	}
}

func TestAgentRefreshCommandRunRefreshes(t *testing.T) {
	refreshProject(t)
	path := filepath.Join(sdtInstrDir, "plan.md")
	writeTestFile(t, path, "<!-- sdt:begin:instructions/plan -->\n# STALE\n<!-- sdt:end:instructions/plan -->\n")

	// Exercise the command wiring itself, not only the shared function: a
	// `force=false` (or a project-block insertion) in the Run body would leave
	// the stale body in place and fail here.
	agentRefreshCmd.Run(agentRefreshCmd, nil)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "# STALE") {
		t.Error("sdt agent refresh did not refresh the drifted generated file")
	}
}

func TestAgentRefreshDoesNotRunTheSeedStep(t *testing.T) {
	cfg := refreshProject(t)
	// The create-only seed files are written by ensureWorkDirs (the init step).
	// Refresh must not run it: a seed that does not exist stays absent, which the
	// content-preservation test above cannot catch (an existing file is skipped
	// by that step anyway).
	seed := ctxTopicsFilePath
	if _, err := os.Stat(seed); !os.IsNotExist(err) {
		t.Fatalf("test setup: %s unexpectedly present", seed)
	}
	if _, err := agentRefreshGenerated(cfg, agentTargetDefault, true, false); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := os.Stat(seed); !os.IsNotExist(err) {
		t.Errorf("refresh created the seed file %s (it ran the init seed step)", seed)
	}
}

func TestAgentRefreshCommandRegisteredUnderAgent(t *testing.T) {
	var found bool
	for _, c := range agentCmd.Commands() {
		if c.Name() == "refresh" {
			found = true
		}
	}
	if !found {
		t.Error("sdt agent refresh is not registered under the agent command")
	}
	for _, surface := range []string{"context/instructions/", "context/commands/", ".agents/skills/"} {
		if !strings.Contains(agentRefreshCmd.Long, surface) {
			t.Errorf("refresh help does not name the %s surface", surface)
		}
	}
}
