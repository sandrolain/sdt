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

// TestMemoSessionStartCheck asserts the SESSION START section runs the due check
// and states the propose-never-block behaviour.
func TestMemoSessionStartCheck(t *testing.T) {
	block := agentBlockInstructions("p", "g")
	if !strings.Contains(block, "sdt context memo due") {
		t.Error("SESSION START must run `sdt context memo due`")
	}
	for _, want := range []string{"before or after the requested task", "never block the task", "never run it\nsilently", "context/instructions/memo.md"} {
		if !strings.Contains(block, want) {
			t.Errorf("SESSION START due check missing %q", want)
		}
	}
}

// TestMemoTriggerResolves proves the >memo document trigger is registered with a
// resolvable contract and that the generated instruction file exists — the same
// lookup an agent performs when the trigger fires.
func TestMemoTriggerResolves(t *testing.T) {
	var found *commandStub
	for i := range agentCommandStubs {
		if agentCommandStubs[i].id == "memo" {
			found = &agentCommandStubs[i]
			break
		}
	}
	if found == nil {
		t.Fatal(">memo is not registered in agentCommandStubs")
	}
	if found.kind != commandKindDocument {
		t.Errorf(">memo kind = %q, want %q", found.kind, commandKindDocument)
	}
	if found.subject != "" {
		t.Errorf(">memo subject = %q, want empty for a document command", found.subject)
	}
	if found.contract != "memo" {
		t.Errorf(">memo contract = %q, want %q", found.contract, "memo")
	}
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	if !generated[found.contract+sdtMarkdownExt] {
		t.Errorf(">memo contract %q has no generated instruction file", found.contract)
	}
}
