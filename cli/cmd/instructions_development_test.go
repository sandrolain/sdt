package cmd

import (
	"strings"
	"testing"
)

func TestDevelopmentInstructionCarriesStableSurfaceAndDegradation(t *testing.T) {
	body := generatedInstruction(t, "development.md")
	flat := strings.Join(strings.Fields(body), " ")

	for _, want := range []string{
		"Keep the always-loaded surface stable", // T1
		"Stable content first",
		"Where the host caches a request prefix",
		"Degrade to pointers, and say what you dropped",
		"Report the omission in the same turn",
		"Name the degradation trigger", // T13
		"invented details that the corpus would have supplied",
		"a session that loads a fixed, small surface has nothing to degrade", // the no-op clause
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated development.md missing %q", want)
		}
	}
}

func TestAgentsBlockStatesTheBoundedProjection(t *testing.T) {
	block := strings.Join(strings.Fields(agentBlockInstructions("p", "g")), " ")
	for _, want := range []string{
		"When the entry point outgrows a session",
		"the rest as pointers",
		"name the tier you omitted",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("AGENTS.md instructions block missing the projection rule %q", want)
		}
	}
}
