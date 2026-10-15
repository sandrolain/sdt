package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAgentVerifyClean(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	out := execute(t, agentVerifyCmd, nil)
	if !strings.Contains(string(out), "OK") {
		t.Errorf("expected OK output: %s", out)
	}
	if !strings.Contains(string(out), "valid") {
		t.Errorf("expected valid message: %s", out)
	}
	jsonOut := execute(t, agentVerifyCmd, nil, "--format", "json")
	if strings.TrimSpace(string(jsonOut)) != "[]" {
		t.Errorf("expected empty json array on clean: %s", jsonOut)
	}
}

func TestAgentVerifyMissingAgents(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if err := os.Remove("AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		out := execute(t, agentVerifyCmd, nil, "--format", "json")
		if !strings.Contains(string(out), "AGENTS.md not found") {
			t.Errorf("expected missing AGENTS.md issue: %s", out)
		}
		return ""
	})
}

func TestAgentVerifyMissingInstructionFile(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	path := "context/instructions/plan.md"
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		out := execute(t, agentVerifyCmd, nil, "--format", "json")
		if !strings.Contains(string(out), "missing instruction file") {
			t.Errorf("expected missing instruction file issue: %s", out)
		}
		return ""
	})
}

func TestAgentVerifyObsoleteFile(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if err := os.WriteFile("context/instructions/memory.md", []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, agentVerifyCmd, nil)
	if !strings.Contains(string(out), "obsolete instruction file") {
		t.Errorf("expected obsolete file warning: %s", out)
	}
}

func TestAgentVerifyMissingWorkDir(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	if err := os.RemoveAll("context/scripts"); err != nil {
		t.Fatal(err)
	}
	out := execute(t, agentVerifyCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "missing work directory") {
		t.Errorf("expected missing work dir warning: %s", out)
	}
}

func TestAgentVerifyFormats(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	for _, format := range []string{"json", "yaml", "text"} {
		out := execute(t, agentVerifyCmd, nil, "--format", format)
		if len(out) == 0 {
			t.Errorf("expected non-empty output for --format %s", format)
		}
	}
}

func TestAgentVerifyBlockMissingReference(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	block := agentBlockInstructions("p", "g")
	block = strings.ReplaceAll(block, "`context/instructions/cli.md`", "`context/instructions/missing.md`")
	block = sectionBlock(agentSectionNameInstructions, block)
	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	start := strings.Index(content, sectionBeginMarker(agentSectionNameInstructions))
	end := strings.Index(content, sectionEndMarker(agentSectionNameInstructions))
	if start < 0 || end < 0 {
		t.Fatal("instructions block not found")
	}
	content = content[:start] + block + content[end+len(sectionEndMarker(agentSectionNameInstructions)):]
	if err := os.WriteFile("AGENTS.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	issues := agentVerifyChecks()
	if err := agentVerifyCriticalError(issues); err == nil || !strings.Contains(err.Error(), "CRITICAL issue(s)") {
		t.Fatalf("expected non-exiting critical result, got %v (issues: %#v)", err, issues)
	}
	foundReference := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "does not reference generated file context/instructions/cli.md") {
			foundReference = true
			break
		}
	}
	if !foundReference {
		t.Fatalf("critical issues did not name the generated file reference: %#v", issues)
	}
	shouldExitWithCode(t, 1, func() string {
		out := execute(t, agentVerifyCmd, nil, "--format", "json")
		if !strings.Contains(string(out), "does not reference generated file") {
			t.Errorf("expected block reference issue: %s", out)
		}
		return ""
	})
}

// TestAgentVerifyFlagsAModuleWithAnH1Title pins the runtime surfacing of the
// generated-set convention. The gate is the template test, which travels with the
// templates; this check is the WARNING an already-installed project sees, and it
// must not fail the command: an installed copy that predates the template has to
// survive a binary upgrade, and the remedy it names — re-run `sdt agent init
// --force` — is the same one the obsolete-file check names.
func TestAgentVerifyFlagsAModuleWithAnH1Title(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")

	const path = "context/instructions/plan.md"
	content := mustReadFile(t, path)
	marker := "<!-- sdt:begin:instructions/plan -->"
	i := strings.Index(content, marker)
	if i < 0 {
		t.Fatalf("module marker missing from %s", path)
	}
	tampered := content[:i+len(marker)] + "\n\n# Plan Documents\n" + content[i+len(marker):]
	writeTestFile(t, path, tampered)

	out := execute(t, agentVerifyCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "markdown H1") {
		t.Errorf("an H1 in a generated module must be reported: %s", out)
	}
	if !strings.Contains(string(out), "sdt agent init --force") {
		t.Errorf("the finding must name the remedy: %s", out)
	}
	// It is a WARNING: the command exits 0, so an installed project is told and
	// not broken.
	if issues := agentVerifyChecks(); agentVerifyCriticalError(issues) != nil {
		t.Errorf("an H1 module must not be critical: %v", agentVerifyCriticalError(issues))
	}
}
