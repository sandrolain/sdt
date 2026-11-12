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
