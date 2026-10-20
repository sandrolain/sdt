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

func TestTasksInstructionCarriesReInjectionAndBatching(t *testing.T) {
	body := generatedInstruction(t, "tasks.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Re-inject the hard rule at a boundary", // F12
		"Restate it at each phase boundary",
		"Batched reads", // T4
		"Broad exploration goes to a sub-agent",
		"a phase that reads two known files needs no batch", // the no-op clause
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated tasks.md missing %q", want)
		}
	}
}

// The replacement note moved to the handoff companion when tasks.md was split:
// it is read when working state is lost, not while executing a step.
func TestTasksHandoffCarriesTheReplacementNote(t *testing.T) {
	body := generatedInstruction(t, "tasks-handoff.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"The replacement note", // T3
		"Failed attempts | every approach tried that did not work",
		"a place that survives the session",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated tasks-handoff.md missing %q", want)
		}
	}
}

// TestWorklogCloseoutIsPerPlan is the R3 gate: the closeout's identity is the
// plan, and a task reference appears only for the split case.
func TestWorklogCloseoutIsPerPlan(t *testing.T) {
	body := generatedInstruction(t, "worklog.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Crystallized closeout (per plan)",
		"**Plan ref**",
		"**Task ref** (split only)",
		"One closeout per plan identity",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("worklog closeout missing %q", want)
		}
	}
	for _, unwanted := range []string{"per phase/task"} {
		if strings.Contains(flat, unwanted) {
			t.Errorf("worklog closeout still assumes a task file: %q", unwanted)
		}
	}
}
