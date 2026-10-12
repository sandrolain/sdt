package cmd

import (
	"strings"
	"testing"
)

func TestCLIInstructionCarriesCorpusFirstAndOutputDiscipline(t *testing.T) {
	body := generatedInstruction(t, "cli.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Search before asking", // F1
		"answered **from the corpus**, not re-asked",
		"Output discipline", // F2
		"Cap what a verb prints",
		"Batch what can be batched",
		"Read the tiered index once",
		"nothing is capped for the sake of a rule", // the no-op clause
		"Help-text shape",                          // F3
		"`WHEN NOT`",
		"Existing command help predates this shape", // convention, not a rewrite
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated cli.md missing %q", want)
		}
	}
}

func TestGitInstructionCarriesWorktreeIsolation(t *testing.T) {
	body := generatedInstruction(t, "git.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Worktrees for parallel work", // F7
		"give it its own worktree",
		"Upgrade trigger",
		"a single session working one branch never needs a worktree", // the no-op clause
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated git.md missing %q", want)
		}
	}
}
