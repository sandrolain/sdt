package cmd

import (
	"strings"
	"testing"
)

// TestWorklogCarriesTheSignalReview guards the bounded self-improvement contract
// after it was folded into the closeout: the four-part signal test, every routing
// destination and the boundaries live in worklog.md, the one file read at the
// moment the loop runs.
func TestWorklogCarriesTheSignalReview(t *testing.T) {
	body := instrWorklogTemplate
	for _, want := range []string{
		"## Signal review",
		"Four-part signal test",
		"Generalisable?",
		"Already recorded?",
		"Which artifact owns the fix?",
		"Which output?",
		"Routing table",
		"context/notes/",
		"note_type: dead-end",
		"Do-Not-Repeat",
		"context/questions/",
		">draft",
		"Observation",
		"HARD RULE 1",
		"`none`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("worklog signal review missing %q", want)
		}
	}
}

// TestSignalReviewHasOneDeliveryPoint proves the fold did not leave a second
// copy: the session-end line points at the closeout section, and no instruction
// module named capture.md survives.
func TestSignalReviewHasOneDeliveryPoint(t *testing.T) {
	block := agentBlockInstructions("p", "g")
	if !strings.Contains(block, "bounded signal check") {
		t.Error("AGENTS.md block must carry the session-end signal check")
	}
	if !strings.Contains(block, "worklog.md") {
		t.Error("AGENTS.md block must point the signal check at worklog.md")
	}
	if strings.Contains(block, "capture.md") {
		t.Error("AGENTS.md block still points at the removed capture.md")
	}
	for _, tmpl := range instructionFiles("", "") {
		if tmpl.name == "capture.md" {
			t.Error("capture.md is still generated")
		}
	}
}
