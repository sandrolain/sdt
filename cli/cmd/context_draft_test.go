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
		"# Draft Capture",
		">draft",
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

func TestDraftCommandStubRegistered(t *testing.T) {
	found := false
	for _, s := range agentCommandStubs {
		if s.id == "draft" {
			found = true
			if s.contract != "draft" {
				t.Errorf("draft contract = %q, want draft", s.contract)
			}
		}
	}
	if !found {
		t.Fatal("draft trigger not registered in agentCommandStubs")
	}

	files := commandFiles("p", contextNow())
	stub := ""
	for _, f := range files {
		if f.name == "draft.md" {
			stub = f.body
		}
	}
	if stub == "" {
		t.Fatal("commandFiles() did not emit the draft stub")
	}
	if !strings.Contains(stub, "instructions/draft.md") {
		t.Errorf("draft stub does not point at its contract:\n%s", stub)
	}

	if idx := commandsIndexContent("p", contextNow()); !strings.Contains(idx, "| `>draft` |") {
		t.Errorf("commands index missing the >draft row:\n%s", idx)
	}
}
