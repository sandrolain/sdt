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
		"Never begin work from an inbox item",
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
