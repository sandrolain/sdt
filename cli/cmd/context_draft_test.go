package cmd

import (
	"strings"
	"testing"
)

func TestDraftInstructionContract(t *testing.T) {
	body := ""
	for _, f := range instructionFiles("p", "g") {
		if f.name == "draft.md" {
			body = f.body
		}
	}
	if body == "" {
		t.Fatal("draft.md not in the generated instruction set")
	}
	for _, want := range []string{
		"title: \"Draft Capture (",
		"verbatim",
		"exactly one",
		"append",
		"status: draft",
		"sdt context new --type analysis",
		"never guess a merge",
		"not migrated",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("draft contract missing %q:\n%s", want, body)
		}
	}
}
