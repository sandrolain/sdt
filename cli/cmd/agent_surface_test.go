package cmd

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/templates"
)

// TestVisibleContextVerbsAreNamed keeps the help and the instruction set honest.
// D3 of the command-defects analysis found nine verbs an agent could read in
// `sdt context --help` and that no generated surface ever named; the decision was
// to deduplicate and keep every verb reachable, not to hide the evidence. This
// test is what makes that a property rather than an intention: a visible verb
// that no surface names fails the build.
func TestVisibleContextVerbsAreNamed(t *testing.T) {
	var surfaces strings.Builder
	for _, tmpl := range templates.List("instructions") {
		surfaces.WriteString(instructionModuleBody(tmpl))
		surfaces.WriteString("\n")
	}
	surfaces.WriteString(agentBlockInstructions("probe", "probe"))
	surfaces.WriteString(commandsIndexContentFor(nil, "probe", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	text := surfaces.String()

	// The rule has to fail on a verb the set does not name, or it can only agree
	// with whatever the surface happens to contain.
	phantom := &cobra.Command{Use: "phantom-verb"}
	if got := unnamedVisibleVerbs([]*cobra.Command{phantom}, text); len(got) != 1 {
		t.Fatalf("the rule accepted an unnamed verb: got %v", got)
	}
	for _, name := range unnamedVisibleVerbs(contextCmd.Commands(), text) {
		t.Errorf("%q is visible in `sdt context --help` but no generated surface names it — name it where its action occurs, or move it out of the agent surface", name)
	}
}

// unnamedVisibleVerbs returns the visible, non-deprecated verbs no generated
// surface names. `help` is cobra's own and is not part of the contract.
func unnamedVisibleVerbs(cmds []*cobra.Command, text string) []string {
	var out []string
	for _, c := range cmds {
		if c.Hidden || c.Deprecated != "" || c.Name() == "help" {
			continue
		}
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(c.Name()) + `\b`).MatchString(text) {
			out = append(out, c.Name())
		}
	}
	return out
}
