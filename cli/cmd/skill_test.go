package cmd

import (
	"strings"
	"testing"
)

func TestSkillCmd_generic(t *testing.T) {
	out := execute(t, skillCmd, nil, "--agent", "generic")
	if !strings.Contains(string(out), "sdt") {
		t.Error("expected 'sdt' in generic skill output")
	}
}

func TestSkillCmd_copilot(t *testing.T) {
	out := execute(t, skillCmd, nil, "--agent", "copilot")
	if !strings.Contains(string(out), "SDT") {
		t.Error("expected 'SDT' in copilot skill output")
	}
}

func TestSkillCmd_claude(t *testing.T) {
	out := execute(t, skillCmd, nil, "--agent", "claude")
	if !strings.Contains(string(out), "tool_instructions") {
		t.Error("expected 'tool_instructions' in claude skill output")
	}
}

func TestSkillCmd_unknownAgent(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, skillCmd, nil, "--agent", "nonexistent-agent-xyz")
		return ""
	})
}

func TestSkillCmd_outputFile(t *testing.T) {
	tmp := t.TempDir() + "/skill_out.md"
	execute(t, skillCmd, nil, "--agent", "generic", "--output", tmp)
	// If no panic/exit, the file was written.
}
