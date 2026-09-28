package cmd

import (
	"strings"
	"testing"
)

func TestScopeInstructionContract(t *testing.T) {
	body := ""
	for _, f := range instructionFiles("p", "g") {
		if f.name == "scope.md" {
			body = f.body
		}
	}
	if body == "" {
		t.Fatal("scope.md not in the generated instruction set")
	}
	for _, want := range []string{
		"# Recursive Design Flow",
		">scope",
		"**Constitution**",
		"Specify (tech-free)",
		"Clarify (recursive)",
		"Plan (technical)",
		"three independent",
		"one question per turn",
		"decided",
		"assumed",
		"deferred",
		"context/questions/",
		"Aperto",
		"Decomposition gate",
		"resumability",
		"status: draft",
		"sdt context new --type analysis",
		"sdt context status set",
		"library-first",
		"HARD RULE 8",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scope contract missing %q:\n%s", want, body)
		}
	}
}

func TestScopeCommandStubRegistered(t *testing.T) {
	found := false
	for _, s := range agentCommandStubs {
		if s.id == "scope" {
			found = true
			if s.contract != "scope" {
				t.Errorf("scope contract = %q, want scope", s.contract)
			}
		}
	}
	if !found {
		t.Fatal("scope trigger not registered in agentCommandStubs")
	}

	files := commandFiles("p", contextNow())
	stub := ""
	for _, f := range files {
		if f.name == "scope.md" {
			stub = f.body
		}
	}
	if stub == "" {
		t.Fatal("commandFiles() did not emit the scope stub")
	}
	if !strings.Contains(stub, "instructions/scope.md") {
		t.Errorf("scope stub does not point at its contract:\n%s", stub)
	}

	if idx := commandsIndexContent("p", contextNow()); !strings.Contains(idx, "| `>scope` |") {
		t.Errorf("commands index missing the >scope row:\n%s", idx)
	}

	if block := agentBlockInstructions("p", "g"); !strings.Contains(block, "`context/instructions/scope.md`") {
		t.Errorf("AGENTS.md instructions block does not reference context/instructions/scope.md")
	}
}
