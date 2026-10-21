package cmd

import (
	"strings"
	"testing"
)

// TestLearningInstructionModule asserts the generated learning.md contract is
// present, non-empty, carries no H1 title and holds the sections the lane needs:
// the no-op rule, the intent table, wiki-first scope resolution with the
// prerequisite rule, the teaching loop, no-solution mode, the citation rule and
// the session close.
func TestLearningInstructionModule(t *testing.T) {
	var body string
	for _, f := range instructionFiles("p", "g") {
		if f.name == "learning.md" {
			body = f.body
			break
		}
	}
	if body == "" {
		t.Fatal("learning.md is not in the generated instruction set")
	}
	if strings.HasPrefix(strings.TrimSpace(body), "# ") {
		t.Error("learning.md must not start with an H1 title")
	}
	for _, want := range []string{
		"**When this applies.**",
		"## Classify the intent",
		"## Resolve the scope",
		"## Teach",
		"## No-solution mode",
		"## Cite",
		"## Close the session",
		"`depends_on`",
		"not in the corpus",
		"context/learning/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("learning.md missing %q", want)
		}
	}
}

// TestLearningTriggersResolve proves every learning trigger is registered as a
// document command whose contract resolves to a generated instruction file and
// declares a payload with at least two examples — the lookup an agent performs
// when the trigger fires.
func TestLearningTriggersResolve(t *testing.T) {
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	byID := map[string]*commandStub{}
	for i := range agentCommandStubs {
		byID[agentCommandStubs[i].id] = &agentCommandStubs[i]
	}

	for _, tc := range []struct{ id, contract string }{
		{"learn", "learning"},
		{"review", "learning"},
		{"quiz", "learning"},
		{"practice", "learning"},
		{"research-learn", "research"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			s := byID[tc.id]
			if s == nil {
				t.Fatalf(">%s is not registered in agentCommandStubs", tc.id)
			}
			if s.kind != commandKindDocument {
				t.Errorf(">%s kind = %q, want %q", tc.id, s.kind, commandKindDocument)
			}
			if s.subject != "" {
				t.Errorf(">%s subject = %q, want empty for a document command", tc.id, s.subject)
			}
			if s.contract != tc.contract {
				t.Errorf(">%s contract = %q, want %q", tc.id, s.contract, tc.contract)
			}
			if !generated[s.contract+sdtMarkdownExt] {
				t.Errorf(">%s contract %q has no generated instruction file", tc.id, s.contract)
			}
			if s.payload == "" {
				t.Errorf(">%s declares no payload", tc.id)
			}
			if len(s.examples) < 2 {
				t.Errorf(">%s has %d example(s), want at least 2", tc.id, len(s.examples))
			}
		})
	}
}
