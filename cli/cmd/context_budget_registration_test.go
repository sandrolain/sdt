//go:build !wasm

package cmd

import (
	"strings"
	"testing"
)

// TestContextBudgetRegisteredInCLICatalog binds the verb to its row in the
// generated command catalog: a verb that ships without its registration fails
// the suite, so the generated `context/instructions/cli.md` can never drift
// behind the shipped surface (phase 2, "registration ships with the artifact").
func TestContextBudgetRegisteredInCLICatalog(t *testing.T) {
	body := instructionTemplateBody("cli.md.tmpl", "p", "g")
	if !strings.Contains(body, "`sdt context budget`") {
		t.Errorf("the generated cli.md template carries no `sdt context budget` row:\n%s", body)
	}
	// The row must state what the report is, so an agent reading only the
	// catalog knows it is a measurement and not a gate.
	for _, want := range []string{"bytes", "informational", "never a gate"} {
		if !strings.Contains(body, want) {
			t.Errorf("the cli.md row does not mention %q, so a reader cannot tell the verb is informational", want)
		}
	}
}

// TestContextBudgetHelpShape locks the declared help shape of the verb, so a
// rewrite that drops the WHEN / WHEN NOT / RETURNS / EXAMPLE contract fails
// here rather than in review.
func TestContextBudgetHelpShape(t *testing.T) {
	long := contextBudgetCmd.Long
	for _, want := range []string{"WHEN —", "WHEN NOT —", "RETURNS —", "EXAMPLE —"} {
		if !strings.Contains(long, want) {
			t.Errorf("the help block is missing the %q section", want)
		}
	}
	if contextBudgetCmd.Short == "" {
		t.Error("the verb carries no Short description")
	}
	for _, want := range []string{"top", "bytes-per-token", "include-modules"} {
		if contextBudgetCmd.Flags().Lookup(want) == nil {
			t.Errorf("the verb declares no --%s flag", want)
		}
	}
}
