package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func instructionFileNames() []string {
	return []string{
		"project.md",
		"analysis.md",
		"plan.md",
		"tasks.md",
		"adr.md",
		"architecture.md",
		"worklog.md",
		"notes.md",
		"questions.md",
		"reference.md",
		"cli.md",
	}
}

func assertInstructionFiles(t *testing.T, dir string) {
	t.Helper()
	for _, f := range instructionFileNames() {
		if _, err := os.Stat(filepath.Join(dir, "sdt.context/instructions", f)); err != nil {
			t.Errorf("expected instruction file %s to be created", f)
		}
	}
}

// ── tagged block helpers ────────────────────────────────────────────────────────

func TestAgentMergeBlock(t *testing.T) {
	content, changed := agentMergeBlock("# H\n", agentSectionNameInstructions, "body", false)
	if !changed {
		t.Error("expected block added")
	}
	if !strings.Contains(content, "<!-- sdt:begin:"+agentSectionNameInstructions+" -->") {
		t.Error("expected instructions block in content")
	}

	again, changed2 := agentMergeBlock(content, agentSectionNameInstructions, "other", false)
	if changed2 || again != content {
		t.Error("expected no change when block exists without force")
	}

	forced, changed3 := agentMergeBlock(content, agentSectionNameInstructions, "refreshed", true)
	if !changed3 || !strings.Contains(forced, "refreshed") {
		t.Error("expected block refreshed with force")
	}
}

func TestAgentAppendIfMissing(t *testing.T) {
	content := agentAppendIfMissing("# H\n", agentSectionNameProject, "body")
	if !strings.Contains(content, "<!-- sdt:begin:"+agentSectionNameProject+" -->") {
		t.Error("expected project block appended when absent")
	}

	again := agentAppendIfMissing(content, agentSectionNameProject, "refreshed-ish-not-forced")
	if again != content {
		t.Error("expected project block unchanged when already present (write-once)")
	}
}

func TestAgentBlockProject(t *testing.T) {
	body := agentBlockProject("myapp", "grp")
	for _, want := range []string{"## Project", "### Stack", "### Build & Run", "### Test", "### Lint & Format", "### Conventions", "<!-- Fill in the sections below. Delete what does not apply. -->"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in project template:\n%s", want, body)
		}
	}
	if strings.Contains(body, "myapp") || strings.Contains(body, "grp") {
		t.Error("project template should not embed project/group identity")
	}
}

func TestAgentMergeTargetProjectWriteOnce(t *testing.T) {
	dir := runInTempDir(t)

	// first init: both blocks created
	res, content := agentMergeTarget("AGENTS.md", "myapp", "grp", false)
	if res.Status != statusCreated {
		t.Fatalf("expected created, got %s", res.Status)
	}
	if !strings.Contains(content, "<!-- sdt:begin:instructions -->") || !strings.Contains(content, "<!-- sdt:begin:project -->") {
		t.Fatal("expected both blocks on first merge")
	}

	// write the file so second merge sees it
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// re-init without force: skip unchanged
	res2, content2 := agentMergeTarget("AGENTS.md", "myapp", "grp", false)
	if res2.Status != statusSkipped {
		t.Fatalf("expected skipped on re-run, got %s", res2.Status)
	}
	if content2 != content {
		t.Error("expected no change on re-run")
	}

	// edit the project block as the user would, then --force must NOT touch it
	edited := strings.Replace(content, "### Stack", "### Stack\n<!-- Go 1.26 -->", 1)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	_, forced := agentMergeTarget("AGENTS.md", "myapp", "grp", true)
	if strings.Contains(forced, "<!-- Go 1.26 -->") != strings.Contains(edited, "<!-- Go 1.26 -->") {
		t.Error("expected --force to leave the project block untouched")
	}
	if !strings.Contains(forced, "<!-- Go 1.26 -->") {
		t.Error("expected user's project block edit preserved through --force")
	}
}

func TestAgentTargetPathFallback(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", sectionBlock("instructions", "x"))
	out := execute(t, agentInitCmd, nil, "--target", "")
	if !strings.Contains(string(out), "[updated] AGENTS.md") {
		t.Errorf("expected fallback to AGENTS.md that adds the project block, got: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "<!-- sdt:begin:project -->") {
		t.Error("expected project block added to legacy instructions-only AGENTS.md")
	}
	if !strings.Contains(string(data), "x") {
		t.Error("expected existing instructions block preserved")
	}
}

func TestAgentReadTargetDirError(t *testing.T) {
	runInTempDir(t)
	if err := os.Mkdir("adir", 0o750); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentInitCmd, nil, "--target", "adir"))
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
	if !strings.Contains(string(data), "<!-- sdt:begin:instructions -->") {
		t.Error("expected instructions block in AGENTS.md")
	}
	if !strings.Contains(string(data), "<!-- sdt:begin:project -->") {
		t.Error("expected project block in AGENTS.md")
	}
	if !strings.Contains(string(data), "<!-- sdt:end:instructions -->") || !strings.Contains(string(data), "<!-- sdt:end:project -->") {
		t.Error("expected instructions and project end markers in AGENTS.md")
	}
	if count := strings.Count(string(data), "<!-- sdt:end:"); count != 2 {
		t.Errorf("expected exactly two tagged blocks (instructions + project), got %d end markers", count)
	}
	for _, want := range []string{
		"sdt.context/instructions/project.md",
		"sdt.context/instructions/analysis.md",
		"sdt.context/instructions/plan.md",
		"sdt.context/instructions/tasks.md",
		"sdt.context/instructions/adr.md",
		"sdt.context/instructions/architecture.md",
		"sdt.context/instructions/worklog.md",
		"sdt.context/instructions/notes.md",
		"sdt.context/instructions/reference.md",
		"sdt.context/instructions/cli.md",
		"### 5-phase development lifecycle",
		"### Knowledge tiers",
		"### Communication (default)",
		"### Patterns (keep updated)",
		"discover them from the",
		"sdt.context/tasks/<phase>.md",
		"sdt.context/architecture/",
		"sdt.context/decisions/",
		"sdt.context/index.md",
		"### Open points (no open questions in analysis/plans)",
		"sdt.context/questions/",
		"### Stack",
		"### Build & Run",
		"### Test",
		"### Lint & Format",
		"### Conventions",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in AGENTS.md index", want)
		}
	}
	if !strings.Contains(string(data), "update the relevant section in the `<!-- sdt:begin:project -->` block") {
		t.Error("expected Patterns to point at the project block")
	}
	if !strings.Contains(string(data), "project: myapp") {
		t.Errorf("expected real project injected in frontmatter:\n%s", data)
	}
	if !strings.Contains(string(data), "group: platform") {
		t.Errorf("expected real group injected in frontmatter:\n%s", data)
	}

	assertInstructionFiles(t, dir)
	proj, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/project.md"))
	for _, want := range []string{"Project: myapp", "Group: platform"} {
		if !strings.Contains(string(proj), want) {
			t.Errorf("expected %q in project.md:\n%s", want, proj)
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
	for _, d := range []string{"sdt.context/plan", "sdt.context/analysis", "sdt.context/worklog", "sdt.context/notes", "sdt.context/questions", "sdt.context/tasks", "sdt.context/archive", "sdt.context/tmp", "sdt.context/instructions", "sdt.context/architecture", "sdt.context/decisions"} {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("expected %s to be created", d)
		}
	}
	assertInstructionFiles(t, dir)
}

func TestAgentInitExisting(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "existing")
	execute(t, agentInitCmd, nil, "--project", "p")
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "existing") {
		t.Error("expected custom content preserved (non-destructive)")
	}
	if !strings.Contains(string(data), "<!-- sdt:begin:instructions -->") {
		t.Error("expected instructions block added to existing AGENTS.md")
	}
	if count := strings.Count(string(data), "<!-- sdt:end:"); count != 2 {
		t.Errorf("expected two tagged blocks (instructions + project), got %d end markers", count)
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
	if !strings.Contains(string(data), "<!-- sdt:begin:instructions -->") {
		t.Error("expected instructions block refreshed with --force")
	}
}

func TestAgentInitEmptyTarget(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, "AGENTS.md", "")
	execute(t, agentInitCmd, nil, "--project", "p")
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "<!-- sdt:begin:instructions -->") {
		t.Error("expected instructions block in generated AGENTS.md")
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
	for _, f := range instructionFileNames() {
		first, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions", f))
		if !strings.Contains(string(first), "#") {
			t.Errorf("expected %s to keep content on second run", f)
		}
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
	if len(res) < 11 {
		t.Errorf("expected config+dirs+instructions+md results, got %d", len(res))
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

func TestAgentInitInstructionsDirError(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "sdt.context/instructions", "file in the way")
	out := execute(t, agentInitCmd, nil, "--project", "p")
	if !strings.Contains(string(out), "[error]") {
		t.Errorf("expected error status for blocked instructions dir: %s", out)
	}
}

func TestAgentInitPreservesInstructionFiles(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	writeTestFile(t, "sdt.context/instructions/adr.md", "custom")
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	data, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/adr.md"))
	if !strings.Contains(string(data), "custom") {
		t.Error("expected custom adr.md preserved without --force")
	}
}

func TestAgentInitForceRefreshesInstructions(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	writeTestFile(t, "sdt.context/instructions/adr.md", "custom")
	execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--force")
	data, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/adr.md"))
	if strings.Contains(string(data), "custom") {
		t.Error("expected adr.md refreshed with --force")
	}
}

func TestAgentInitForceRemovesObsoleteInstructions(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	for _, name := range obsoleteInstructionFiles {
		writeTestFile(t, filepath.Join("sdt.context/instructions", name), "obsolete")
	}
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--force")
	if !strings.Contains(string(out), "removed") {
		t.Errorf("expected removed status for obsolete files: %s", out)
	}
	for _, name := range obsoleteInstructionFiles {
		if _, err := os.Stat(filepath.Join(dir, "sdt.context/instructions", name)); !os.IsNotExist(err) {
			t.Errorf("expected obsolete instruction file %s to be removed with --force", name)
		}
	}
}

// ── .gitignore handling ────────────────────────────────────────────────────────

func TestEnsureGitIgnoreNoParentResolution(t *testing.T) {
	parent := runInTempDir(t)
	writeTestFile(t, ".gitignore", "from-parent/\n")
	sub := filepath.Join(parent, "child")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil || res.Status != statusCreated {
		t.Fatalf("expected created in current dir (no parent resolution), got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(sub, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("expected entries in local .gitignore:\n%s", data)
	}
	if parentData, err := os.ReadFile(filepath.Join(parent, ".gitignore")); err != nil || strings.Contains(string(parentData), gitIgnoreTmpEntry) {
		t.Errorf("expected parent .gitignore untouched:\n%s", parentData)
	}
}

func TestEnsureGitIgnoreUsesLocalDir(t *testing.T) {
	parent := runInTempDir(t)
	writeTestFile(t, ".gitignore", "from-parent/\n")
	sub := filepath.Join(parent, "child")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "local-preexisting/\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil || res.Status != statusUpdated {
		t.Fatalf("expected updated in current dir, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(sub, ".gitignore"))
	if !strings.Contains(string(data), "local-preexisting/") || !strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("expected local entries:\n%s", data)
	}
	parentData, _ := os.ReadFile(filepath.Join(parent, ".gitignore"))
	if !strings.Contains(string(parentData), "from-parent/") || strings.Contains(string(parentData), gitIgnoreTmpEntry) {
		t.Errorf("expected parent .gitignore untouched:\n%s", parentData)
	}
}

func TestEnsureGitIgnoreWithoutGit(t *testing.T) {
	runInTempDir(t)
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil || res.Status != statusCreated {
		t.Fatalf("expected created without git repo, got %+v", res)
	}
}

func TestEnsureGitIgnoreCreate(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil {
		t.Fatal("expected result for git repo")
	}
	if res.Status != statusCreated {
		t.Errorf("expected created, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreTmpEntry) || !strings.Contains(string(data), gitIgnoreDocsEntry) {
		t.Errorf("expected %q and %q in .gitignore:\n%s", gitIgnoreTmpEntry, gitIgnoreDocsEntry, data)
	}
	if !strings.HasPrefix(string(data), gitIgnoreBlockStart+"\n") || !strings.HasSuffix(string(data), gitIgnoreBlockEnd+"\n") {
		t.Errorf("expected entries wrapped in # sdt:start / # sdt:end:\n%s", data)
	}
}

func TestEnsureGitIgnoreUpdate(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "bin/\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil {
		t.Fatal("expected result for git repo")
	}
	if res.Status != statusUpdated {
		t.Errorf("expected updated, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), "bin/") || !strings.Contains(string(data), gitIgnoreTmpEntry) || !strings.Contains(string(data), gitIgnoreDocsEntry) {
		t.Errorf("expected existing and new entries in .gitignore:\n%s", data)
	}
}

func TestEnsureGitIgnoreAddsMissingEntry(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", gitIgnoreTmpEntry+"\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil {
		t.Fatal("expected result for git repo")
	}
	if res.Status != statusUpdated {
		t.Errorf("expected updated when one entry is missing, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreDocsEntry) {
		t.Errorf("expected %q added:\n%s", gitIgnoreDocsEntry, data)
	}
	if strings.Count(string(data), gitIgnoreTmpEntry) != 1 {
		t.Errorf("expected no duplicate tmp entry:\n%s", data)
	}
}

func TestEnsureGitIgnoreSkip(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "bin/\n"+gitIgnoreTmpEntry+"\n"+gitIgnoreDocsEntry+"\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil {
		t.Fatal("expected result for git repo")
	}
	if res.Status != statusSkipped {
		t.Errorf("expected skipped, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(data), gitIgnoreTmpEntry) != 1 {
		t.Errorf("expected no duplicate entry in .gitignore:\n%s", data)
	}
}

func TestEnsureGitIgnoreSkipNoSlash(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "sdt.context/tmp\nsdt.context/docs\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil {
		t.Fatal("expected result for git repo")
	}
	if res.Status != statusSkipped {
		t.Errorf("expected skipped for entries without trailing slash, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(data), "sdt.context/tmp/") || strings.Contains(string(data), "sdt.context/docs/") {
		t.Errorf("expected no duplicate entries in .gitignore:\n%s", data)
	}
}

func TestEnsureGitIgnorePreservesTrailingContent(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "bin/")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil || res.Status != statusUpdated {
		t.Fatalf("expected updated, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	body := string(data)
	if !strings.HasPrefix(body, "bin/\n") {
		t.Errorf("expected existing content preserved:\n%s", body)
	}
	if !strings.HasSuffix(body, gitIgnoreBlockStart+"\n"+gitIgnoreTmpEntry+"\n"+gitIgnoreDocsEntry+"\n"+gitIgnoreBlockEnd+"\n") {
		t.Errorf("expected sdt block appended with markers:\n%s", body)
	}
}

func TestEnsureGitIgnoreAddsMissingInsideBlock(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ".gitignore", "bin/\n"+gitIgnoreBlockStart+"\n"+gitIgnoreTmpEntry+"\n"+gitIgnoreBlockEnd+"\n")
	res := ensureGitIgnore(gitIgnoreModeWork)
	if res == nil || res.Status != statusUpdated {
		t.Fatalf("expected updated, got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	body := string(data)
	if strings.Count(body, gitIgnoreDocsEntry) != 1 {
		t.Errorf("expected docs entry added inside block:\n%s", body)
	}
	if !strings.Contains(body, gitIgnoreBlockStart+"\n"+gitIgnoreTmpEntry+"\n"+gitIgnoreDocsEntry+"\n"+gitIgnoreBlockEnd) {
		t.Errorf("expected missing entry inserted inside the sdt block:\n%s", body)
	}
	if strings.Count(body, gitIgnoreBlockEnd) != 1 {
		t.Errorf("expected single end marker:\n%s", body)
	}
	if strings.Count(body, gitIgnoreTmpEntry) != 1 {
		t.Errorf("expected no duplicate tmp entry:\n%s", body)
	}
}

func TestAgentInitAddsGitIgnore(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if !strings.Contains(string(out), ".gitignore") {
		t.Errorf("expected .gitignore in init output: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("expected %q in .gitignore:\n%s", gitIgnoreTmpEntry, data)
	}
}

func TestAgentInitGitIgnoreIdempotent(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if !strings.Contains(string(out), "skipped") {
		t.Errorf("expected skipped status on second run: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(data), gitIgnoreTmpEntry) != 1 {
		t.Errorf("expected exactly one entry in .gitignore:\n%s", data)
	}
}

func TestAgentInitAddsGitIgnoreWithoutGit(t *testing.T) {
	dir := runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if !strings.Contains(string(out), ".gitignore") {
		t.Errorf("expected .gitignore handling even outside git repo: %s", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("expected entries in .gitignore:\n%s", data)
	}
}

func TestAgentInitGitIgnoreNone(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	out := execute(t, agentInitCmd, nil, "--project", "p", "--gitignore", gitIgnoreModeNone)
	if strings.Contains(string(out), ".gitignore") {
		t.Errorf("expected no .gitignore handling with --gitignore none: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error("expected no .gitignore created with --gitignore none")
	}
}

func TestAgentInitGitIgnoreTmp(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--gitignore", gitIgnoreModeTmp)
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("expected %q in .gitignore:\n%s", gitIgnoreTmpEntry, data)
	}
	if strings.Contains(string(data), gitIgnoreDocsEntry) {
		t.Errorf("did not expect %q with --gitignore tmp:\n%s", gitIgnoreDocsEntry, data)
	}
}

func TestAgentInitGitIgnoreDocs(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--gitignore", gitIgnoreModeDocs)
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreDocsEntry) {
		t.Errorf("expected %q in .gitignore:\n%s", gitIgnoreDocsEntry, data)
	}
	if strings.Contains(string(data), gitIgnoreTmpEntry) {
		t.Errorf("did not expect %q with --gitignore docs:\n%s", gitIgnoreTmpEntry, data)
	}
}

func TestAgentInitGitIgnoreContext(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Mkdir(".git", 0o750); err != nil {
		t.Fatal(err)
	}
	execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--gitignore", gitIgnoreModeContext)
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), gitIgnoreContextEntry) {
		t.Errorf("expected %q in .gitignore:\n%s", gitIgnoreContextEntry, data)
	}
}

func TestAgentInitGitIgnoreInvalid(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentInitCmd, nil, "--project", "p", "--gitignore", "bogus"))
	})
}

func TestGitIgnoreEntriesForMode(t *testing.T) {
	cases := map[string][]string{
		gitIgnoreModeNone:    nil,
		gitIgnoreModeTmp:     {gitIgnoreTmpEntry},
		gitIgnoreModeDocs:    {gitIgnoreDocsEntry},
		gitIgnoreModeWork:    {gitIgnoreTmpEntry, gitIgnoreDocsEntry},
		gitIgnoreModeContext: {gitIgnoreContextEntry},
		"":                   {gitIgnoreTmpEntry, gitIgnoreDocsEntry},
	}
	for mode, want := range cases {
		got := gitIgnoreEntriesForMode(mode)
		if len(got) != len(want) {
			t.Errorf("mode %q: expected %v entries, got %v", mode, want, got)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("mode %q entry %d: expected %q, got %q", mode, i, want[i], got[i])
			}
		}
	}
}

func TestResolveGitIgnoreModeFlag(t *testing.T) {
	for _, mode := range []string{gitIgnoreModeNone, gitIgnoreModeTmp, gitIgnoreModeDocs, gitIgnoreModeWork, gitIgnoreModeContext} {
		cmd := &cobra.Command{}
		cmd.Flags().String("gitignore", "", "")
		if err := cmd.Flags().Set("gitignore", mode); err != nil {
			t.Fatal(err)
		}
		if got := resolveGitIgnoreMode(cmd, false); got != mode {
			t.Errorf("flag %q: expected %q, got %q", mode, mode, got)
		}
	}
}

func TestResolveGitIgnoreModeFlagCaseInsensitive(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("gitignore", "", "")
	if err := cmd.Flags().Set("gitignore", "WORK"); err != nil {
		t.Fatal(err)
	}
	if got := resolveGitIgnoreMode(cmd, false); got != gitIgnoreModeWork {
		t.Errorf("expected work for uppercase flag, got %q", got)
	}
}

func TestResolveGitIgnoreModeDefault(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("gitignore", "", "")
	if got := resolveGitIgnoreMode(cmd, true); got != gitIgnoreModeWork {
		t.Errorf("expected work default with --yes, got %q", got)
	}
}

func TestResolveGitIgnoreModeNonTTY(t *testing.T) {
	if !stdinIsTTY() {
		cmd := &cobra.Command{}
		cmd.Flags().String("gitignore", "", "")
		if got := resolveGitIgnoreMode(cmd, false); got != gitIgnoreModeWork {
			t.Errorf("expected work default without TTY, got %q", got)
		}
	}
}

func TestResolveGitIgnoreModeInvalid(t *testing.T) {
	runInTempDir(t)
	cmd := &cobra.Command{}
	cmd.Flags().String("gitignore", "", "")
	if err := cmd.Flags().Set("gitignore", "bogus"); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		return string(resolveGitIgnoreMode(cmd, false))
	})
}

func TestAgentPromptBool(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	defer devNull.Close()
	orig := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = orig }()

	if !stdinIsTTY() {
		t.Skip("/dev/null is not a character device on this system")
	}
	cases := []struct {
		in  string
		def bool
		got bool
	}{
		{"y\n", true, true},
		{"n\n", true, false},
		{"yes\n", false, true},
		{"NO\n", true, false},
		{"\n", true, true},
		{"junk\n", true, true},
	}
	for _, c := range cases {
		cmd := &cobra.Command{}
		cmd.SetIn(strings.NewReader(c.in))
		cmd.SetErr(failingWriter{})
		if got := agentPromptBool(cmd, false, "label", c.def); got != c.got {
			t.Errorf("input %q def %v: expected %v, got %v", c.in, c.def, c.got, got)
		}
	}
}

// withTTYPrompts runs fn with os.Stdin pointing at a character device so that
// prompt helpers take the interactive branch. It spawns a helper process: the
// standard trick of reading /dev/null works for other suites but here stdin
// content is driven through cmd.SetIn.
func withTTYPrompt(t *testing.T, fn func(cmd *cobra.Command)) {
	t.Helper()
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	defer devNull.Close()
	orig := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = orig }()
	if !stdinIsTTY() {
		t.Skip("/dev/null is not a character device on this system")
	}
	cmd := &cobra.Command{}
	cmd.SetErr(failingWriter{})
	fn(cmd)
}

func TestAgentPromptGitIgnoreModeParsing(t *testing.T) {
	withTTYPrompt(t, func(cmd *cobra.Command) {
		cases := []struct {
			in   string
			want string
		}{
			{"1\n", gitIgnoreModeTmp},
			{"2\n", gitIgnoreModeDocs},
			{"1,2\n", gitIgnoreModeWork},
			{"3\n", gitIgnoreModeContext},
			{"tmp, docs\n", gitIgnoreModeWork},
			{"context\n", gitIgnoreModeContext},
			{"work\n", gitIgnoreModeWork},
			{"\n", gitIgnoreModeWork},
			{"garbage\n", gitIgnoreModeWork},
			{"5\n", gitIgnoreModeWork},
		}
		for _, c := range cases {
			cmd.SetIn(strings.NewReader(c.in))
			if got := agentPromptGitIgnoreMode(cmd, false, gitIgnoreModeWork); got != c.want {
				t.Errorf("input %q: expected %q, got %q", c.in, c.want, got)
			}
		}
	})
}

func TestAgentPromptGitIgnoreModeYes(t *testing.T) {
	if got := agentPromptGitIgnoreMode(&cobra.Command{}, true, gitIgnoreModeWork); got != gitIgnoreModeWork {
		t.Errorf("expected default with yes, got %q", got)
	}
}

func TestAgentInitYAMLOutput(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--format", "yaml")
	if !strings.Contains(string(out), "path:") {
		t.Errorf("expected yaml list output: %s", out)
	}
}

func TestAgentInstructionsBlock(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")

	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	for _, want := range []string{
		"### 5-phase development lifecycle",
		"### Knowledge tiers",
		"### Communication (default)",
		"### Patterns (keep updated)",
		"caveman ultra",
		"[thing] [action] [reason]",
		"Conventional Commits",
		"≤50 chars",
		"sdt.context/",
		"sdt.context/tasks/<phase>.md",
		"sdt.context/architecture/",
		"sdt.context/decisions/",
		"sdt.context/index.md",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in AGENTS.md instructions block:\n%s", want, data)
		}
	}

	for _, name := range []string{
		"README.md",
		"commands.md",
		"workflow.md",
		"communication.md",
		"planning.md",
		"annotations.md",
		"self-update.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, "sdt.context/instructions", name)); !os.IsNotExist(err) {
			t.Errorf("expected obsolete instruction file %s to be absent", name)
		}
	}

	proj, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/project.md"))
	if strings.Contains(string(proj), "## Project Configuration") {
		t.Errorf("expected project.md without Project Configuration section:\n%s", proj)
	}

	readme, _ := os.ReadFile(filepath.Join(dir, "sdt.context/README.md"))
	if !strings.Contains(string(readme), "instructions/") {
		t.Errorf("expected instructions/ layout in sdt.context/README.md:\n%s", readme)
	}

	ref, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/reference.md"))
	if !strings.Contains(string(ref), "sdt agent init") {
		t.Errorf("expected reference.md to cover agent init:\n%s", ref)
	}
	if !strings.Contains(string(ref), "sdt manifest --format json") {
		t.Errorf("expected reference.md to point to manifest:\n%s", ref)
	}

	adr, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/adr.md"))
	for _, want := range []string{"decisions/", "NNNN-", "append-only", "sync", "architecture/"} {
		if !strings.Contains(string(adr), want) {
			t.Errorf("expected %q in adr.md:\n%s", want, adr)
		}
	}

	arch, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/architecture.md"))
	for _, want := range []string{"kebab-case", "no date", "Mermaid", "essential"} {
		if !strings.Contains(string(arch), want) {
			t.Errorf("expected %q in architecture.md:\n%s", want, arch)
		}
	}

	plan, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/plan.md"))
	for _, want := range []string{"5-phase", "Analysis", "Tasks", "living"} {
		if !strings.Contains(string(plan), want) {
			t.Errorf("expected %q in plan.md:\n%s", want, plan)
		}
	}

	proj, _ = os.ReadFile(filepath.Join(dir, "sdt.context/instructions/project.md"))
	for _, want := range []string{"## Identity", "--project", "no implicit fallback", "<dirname>_<short-path-hash>"} {
		if !strings.Contains(string(proj), want) {
			t.Errorf("expected %q in project.md:\n%s", want, proj)
		}
	}

	cli, _ := os.ReadFile(filepath.Join(dir, "sdt.context/instructions/cli.md"))
	for _, want := range []string{"--format text|json|yaml", "sdt conv --in json --out yaml", "sdt context new|path|list|task|docs", "sdt diff --a A --b B --diff-format", "sdt dns --host"} {
		if !strings.Contains(string(cli), want) {
			t.Errorf("expected %q in cli.md:\n%s", want, cli)
		}
	}
}

func TestAgentBlockInstructionsFrontmatter(t *testing.T) {
	with := agentBlockInstructions("myapp", "platform")
	if !strings.Contains(with, "project: myapp\n") {
		t.Errorf("expected injected project:\n%s", with)
	}
	if !strings.Contains(with, "group: platform\n") {
		t.Errorf("expected injected group:\n%s", with)
	}
	without := agentBlockInstructions("", "")
	if !strings.Contains(without, "project: <project>\n") {
		t.Errorf("expected placeholder fallback when project empty:\n%s", without)
	}
	if strings.Contains(without, "group:") {
		t.Errorf("expected no group line when group empty:\n%s", without)
	}
}

func TestAgentPromptNonTTY(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if !strings.Contains(string(out), "[written]") {
		t.Errorf("expected init output: %s", out)
	}
}

func hasErrorStatus(results []FileResult) bool {
	for _, r := range results {
		if r.Status == statusError {
			return true
		}
	}
	return false
}

// ── error paths ────────────────────────────────────────────────────────────────

func TestWriteInstructionFilesMkdirError(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if res := writeInstructionFiles("p", "g", false); !hasErrorStatus(res) {
		t.Fatalf("expected mkdir error status, got %+v", res)
	}
}

func TestWriteInstructionFilesWriteError(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "sdt.context/instructions/.keep", "")
	if err := os.Chmod("sdt.context/instructions", 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod("sdt.context/instructions", 0o755) })
	if res := writeInstructionFiles("p", "g", false); !hasErrorStatus(res) {
		t.Fatalf("expected write error status, got %+v", res)
	}
}

func TestEnsureWorkDirsMkdirError(t *testing.T) {
	dir := runInTempDir(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if res := ensureWorkDirs(false); !hasErrorStatus(res) {
		t.Fatalf("expected mkdir error status, got %+v", res)
	}
}

func TestEnsureWorkDirsReadmeError(t *testing.T) {
	runInTempDir(t)
	for _, d := range []string{sdtWorkDir, sdtPlanDir, sdtWorklogDir, sdtNotesDir, sdtTasksDir, sdtArchiveDir, sdtTmpDir, sdtInstrDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(sdtWorkDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sdtWorkDir, 0o755) })
	if res := ensureWorkDirs(false); !hasErrorStatus(res) {
		t.Fatalf("expected readme error status, got %+v", res)
	}
}

func TestAgentPromptTTY(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	defer devNull.Close()
	orig := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = orig }()

	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("answer\n"))
	cmd.SetErr(&bytes.Buffer{})

	if !stdinIsTTY() {
		t.Skip("/dev/null is not a character device on this system")
	}
	if got := agentPrompt(cmd, false, "label", "def"); got != "answer" {
		t.Errorf("expected prompt to read from stdin, got %q", got)
	}
}

func TestAgentPromptReadError(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	defer devNull.Close()
	orig := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = orig }()

	cmd := &cobra.Command{}
	cmd.SetIn(failingReader{})
	cmd.SetErr(failingWriter{})

	if !stdinIsTTY() {
		t.Skip("/dev/null is not a character device on this system")
	}
	if got := agentPrompt(cmd, false, "label", "def"); got != "def" {
		t.Errorf("expected default on read error, got %q", got)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failure") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestStdinIsTTYError(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("cannot open /dev/null")
	}
	_ = f.Close()
	orig := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = orig }()
	if stdinIsTTY() {
		t.Error("expected false for closed stdin")
	}
}

func TestAgentInitConfigWriteError(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml/blocker", "")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentInitCmd, nil, "--project", "p", "--yes"))
	})
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
