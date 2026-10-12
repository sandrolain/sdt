package cmd

import (
	"strings"
	"testing"
)

// generatedInstruction returns the generated body of one instruction file, or
// fails when the template directory no longer produces it.
func generatedInstruction(t *testing.T, name string) string {
	t.Helper()
	for _, file := range instructionFiles("p", "g") {
		if file.name == name {
			return file.body
		}
	}
	t.Fatalf("generated instruction %s not found", name)
	return ""
}

func TestDiagramInstructionCarriesBudgetHonestyProvenanceAndTarget(t *testing.T) {
	body := generatedInstruction(t, "diagram.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Budget the density",                // complexity budget (F4)
		"split the picture or draw nothing", // the escalation, not a smaller type
		"Upgrade trigger",                   // the shortcut marker development.md defines
		"Quantitative axes are never truncated",
		"does not establish order, downtime or causality",
		"Never invent a component to fill a layout, and never silently drop one",
		"Structure has a provenance", // structure provenance (F15)
		"code layer is not drawn by hand",
		"plain-text form where the consumer has no renderer", // the target ladder (F17)
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated diagram.md missing %q", want)
		}
	}

	// The ladder states a per-artefact decision; the module still names no tool.
	if strings.Contains(flat, "renderer such as") || strings.Contains(flat, "install the") {
		t.Error("the diagram module must stay tool-neutral")
	}
}
