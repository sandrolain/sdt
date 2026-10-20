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

func TestDevelopmentInstructionCarriesLiveOutputDestination(t *testing.T) {
	flat := strings.Join(strings.Fields(generatedInstruction(t, "development.md")), " ")

	for _, want := range []string{
		"Send an output somewhere, do not print it into the thread",
		"context/tmp/",                                           // the throwaway destination
		"context/refs/<topic>/",                                  // the citable destination
		"an agent cannot delete bytes already in",                // the claim is an avoidance, not an erasure
		"reduce an artefact *entering* context",                  // how it relates to the compression rules
		"write it to a file only when it would crowd the thread", // the no-op clause
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("generated development.md missing %q", want)
		}
	}
}

func TestDevelopmentInstructionCarriesOutlineFirstRead(t *testing.T) {
	flat := strings.Join(strings.Fields(generatedInstruction(t, "development.md")), " ")

	for _, want := range []string{
		"Read the structure first, unfold the range you need",
		"sdt read --lines <from>:<to>",
		"re-reading a file that was already read", // the symptom the rule prevents
		"Two levels, not a repeat",                // the clause names both levels
		"the essential entries inline and the rest as pointers",
		"Take the neighbourhood in one call", // the honest cost
		"If it fits in one read, read it",    // the no-op clause
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
