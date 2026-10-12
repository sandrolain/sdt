package cmd

import (
	"strings"
	"testing"
)

func TestPlanInstructionCarriesRequirementsDecompositionAndBlindReader(t *testing.T) {
	body := generatedInstruction(t, "plan.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"From a requirements document to phases", // F8
		"name the **inter-phase dependency**",
		"elicit the quality gates explicitly",
		"a small, direct objective with no requirements", // the no-op clause
		"blind reader-test",                              // F14
		"**fresh context** with no shared working state",
		"**deferred, not skipped**", // the degraded path stays visible
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated plan.md missing %q", want)
		}
	}
}

func TestTasksInstructionCarriesReInjectionBatchingAndReplacementNote(t *testing.T) {
	body := generatedInstruction(t, "tasks.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Re-inject the hard rule at a boundary", // F12
		"Restate it at each phase boundary",
		"Batched reads", // T4
		"Broad exploration goes to a sub-agent",
		"a phase that reads two known files needs no batch", // the no-op clause
		"The replacement note",                              // T3
		"Failed attempts | every approach tried that did not work",
		"a place that survives the session",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated tasks.md missing %q", want)
		}
	}
}
