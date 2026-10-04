package cmd

import (
	"strings"
	"testing"
)

// TestMemoInstructionModule asserts the generated memo.md contract is present,
// non-empty and carries its required doctrine: the rule/memo model, the
// CLI-owned register, the opportunistic due check, propose-never-block,
// done-only-after-success and no H1 title.
func TestMemoInstructionModule(t *testing.T) {
	var body string
	for _, f := range instructionFiles("p", "g") {
		if f.name == "memo.md" {
			body = f.body
			break
		}
	}
	if body == "" {
		t.Fatal("memo.md is not in the generated instruction set")
	}
	if strings.HasPrefix(strings.TrimSpace(body), "# ") {
		t.Error("memo.md must not start with an H1 title")
	}
	for _, want := range []string{
		"## The register",
		"## The due check",
		"## Acting on a due item",
		"## Boundaries",
		"CLI-owned",
		"`last_done + every_days <= today`",
		"`today >= due`",
		"sdt context memo due",
		"sdt context memo done",
		"Propose, never block or run silently",
		"One operation per run",
		"Record `done` only after success",
		"never enforces",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("memo.md missing %q", want)
		}
	}
}

// TestMemoInstructionDiscoverable asserts the AGENTS.md instructions block wires
// the memo contract into the On-action table.
func TestMemoInstructionDiscoverable(t *testing.T) {
	block := agentBlockInstructions("p", "g")
	hasRow := false
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, "context/instructions/memo.md") &&
			strings.Contains(line, "Planned operations") {
			hasRow = true
			break
		}
	}
	if !hasRow {
		t.Error("expected an On-action row for context/instructions/memo.md")
	}
}
