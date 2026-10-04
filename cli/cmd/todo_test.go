package cmd

import (
	"strings"
	"testing"
)

// TestTodoInstructionModule asserts the generated todo.md contract is present,
// non-empty and carries its required content: the annotate-only rule, the
// CLI-owned register, the verbs and the never-start-work boundary, with no H1.
func TestTodoInstructionModule(t *testing.T) {
	var body string
	for _, f := range instructionFiles("p", "g") {
		if f.name == "todo.md" {
			body = f.body
			break
		}
	}
	if body == "" {
		t.Fatal("todo.md is not in the generated instruction set")
	}
	if strings.HasPrefix(strings.TrimSpace(body), "# ") {
		t.Error("todo.md must not start with an H1 title")
	}
	for _, want := range []string{
		"## The inbox",
		"Annotate only",
		"Never begin work from `>todo`",
		"`sdt context todo add",
		"`sdt context todo done <id>`",
		"## Boundaries",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("todo.md missing %q", want)
		}
	}
}

// TestTodoInstructionDiscoverable asserts the AGENTS.md instructions block wires
// the inbox contract into the On-action table.
func TestTodoInstructionDiscoverable(t *testing.T) {
	block := agentBlockInstructions("p", "g")
	if !strings.Contains(block, "`context/instructions/todo.md`") {
		t.Error("AGENTS.md block does not reference context/instructions/todo.md")
	}
}

// TestTodoTriggerResolves proves the >todo document trigger is registered with a
// resolvable contract and that the generated instruction file exists — the same
// lookup an agent performs when the trigger fires.
func TestTodoTriggerResolves(t *testing.T) {
	var found *commandStub
	for i := range agentCommandStubs {
		if agentCommandStubs[i].id == "todo" {
			found = &agentCommandStubs[i]
			break
		}
	}
	if found == nil {
		t.Fatal(">todo is not registered in agentCommandStubs")
	}
	if found.kind != commandKindDocument {
		t.Errorf(">todo kind = %q, want %q", found.kind, commandKindDocument)
	}
	if found.subject != "" {
		t.Errorf(">todo subject = %q, want empty for a document command", found.subject)
	}
	if found.contract != "todo" {
		t.Errorf(">todo contract = %q, want %q", found.contract, "todo")
	}
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	if !generated[found.contract+sdtMarkdownExt] {
		t.Errorf(">todo contract %q has no generated instruction file", found.contract)
	}
}
