package cmd

import (
	"strings"
	"testing"
)

// TestCaptureInstructionModule guards the bounded self-improvement capture
// contract: it renders, carries the four-part signal test and every routing
// destination, and — per the document convention — has no H1 title.
func TestCaptureInstructionModule(t *testing.T) {
	body := instrCaptureTemplate
	if strings.TrimSpace(body) == "" {
		t.Fatal("capture template rendered empty")
	}
	for _, want := range []string{
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
		"approval gate",
		"`none`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("capture template missing %q", want)
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			t.Errorf("capture template must not carry an H1 title: %q", line)
		}
	}
}

// TestCaptureDeliveryPoints guards that both delivery points — the worklog
// crystallized closeout and the AGENTS.md session end — point at capture.md
// instead of restating its routing table.
func TestCaptureDeliveryPoints(t *testing.T) {
	if !strings.Contains(instrWorklogTemplate, "capture.md") {
		t.Error("worklog closeout must reference the capture contract")
	}
	if !strings.Contains(instrWorklogTemplate, "Signal review") {
		t.Error("worklog closeout must carry a Signal review line")
	}
	block := agentBlockInstructions("p", "g")
	if !strings.Contains(block, "context/instructions/capture.md") {
		t.Error("AGENTS.md block must reference the capture contract")
	}
	if !strings.Contains(block, "bounded signal check") {
		t.Error("AGENTS.md block must carry the session-end signal check")
	}
}
