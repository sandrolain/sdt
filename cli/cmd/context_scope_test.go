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
		"title: \"Recursive Design Flow (",
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
		"Open points",
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
