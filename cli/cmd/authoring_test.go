package cmd

import (
	"strings"
	"testing"
)

// TestAuthoringInstructionModule guards the authoring contract: it renders and
// carries the core rules (trigger-not-summary descriptions, word budgets, no
// force-include, RED-first authoring, form-to-failure, no-guidance control). Per
// the document convention the template carries no H1 title.
func TestAuthoringInstructionModule(t *testing.T) {
	body := instrAuthoringTemplate
	if strings.TrimSpace(body) == "" {
		t.Fatal("authoring template rendered empty")
	}
	for _, want := range []string{
		"when to use",
		"Word budget",
		"force-include a file",
		"form-to-failure",
		"no-guidance control",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("authoring template missing %q", want)
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			t.Errorf("authoring template must not carry an H1 title: %q", line)
		}
	}
}
