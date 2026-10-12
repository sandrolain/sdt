package cmd

import (
	"strings"
	"testing"
)

func TestWikiAndIngestionCarryTheCorrectionSweep(t *testing.T) {
	wiki := strings.Join(strings.Fields(generatedInstruction(t, "wiki.md")), " ")
	ingestion := strings.Join(strings.Fields(generatedInstruction(t, "ingestion.md")), " ")

	for _, want := range []string{
		"The correction sweep",
		"Find the restatements",
		"A person names the authoritative statement",
		"living restatement",
		"historical record",
		"Report a reviewable change set before writing",
		"that is a real outcome, not a failure", // the no-authority stop
		"Upgrade trigger",
	} {
		if !strings.Contains(wiki, want) {
			t.Errorf("generated wiki.md missing the sweep clause %q", want)
		}
	}

	for _, want := range []string{
		"run the correction sweep",
		"A source is evidence, not an override",
	} {
		if !strings.Contains(ingestion, want) {
			t.Errorf("generated ingestion.md missing %q", want)
		}
	}

	// The pre-existing rule the sweep implements is preserved, not replaced.
	if !strings.Contains(wiki, "Never overwrite an existing claim silently") {
		t.Error("the correction sweep replaced the never-overwrite rule instead of implementing it")
	}
}
