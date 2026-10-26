package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCloseoutCopiesReviewVerdictsVerbatim pins the fix for the flattening: a
// continuation line belongs to its finding and must not become a bullet of its
// own.
func TestCloseoutCopiesReviewVerdictsVerbatim(t *testing.T) {
	body := "## Review\n\n- **Phase 1 — CONFIRMED.** first line\n  second line\n- **Phase 2 — CONFIRMED.** third\n\n### Gate\n\n2026-01-01T00:00:00Z (sdt agent gate) — passed\n"
	_, verdicts, gate := closeoutScanLedger(body)
	if len(verdicts) != 3 {
		t.Fatalf("verdicts = %d, want 3: %#v", len(verdicts), verdicts)
	}
	if gate == "" {
		t.Fatal("gate run not read")
	}
	rendered := closeoutFactsRender(closeoutFacts{Verdicts: verdicts, Slug: "s", PlanRef: "plan/x.md"}, "2026-01-01", nil)
	if !strings.Contains(rendered, "  - **Phase 1 — CONFIRMED.** first line\n    second line\n") {
		t.Errorf("verdict structure lost:\n%s", rendered)
	}
	if strings.Contains(rendered, "\n  - second line") {
		t.Errorf("continuation line became its own finding:\n%s", rendered)
	}
}

// TestCloseoutCarriesOnlyDeclaredFields proves a bold verdict line is never read
// back as a human field, which used to duplicate the whole review on a rerun.
func TestCloseoutCarriesOnlyDeclaredFields(t *testing.T) {
	block := "- **Files affected**: a.md\n- **Phase 1 — CONFIRMED.**: not a field\n- **Signal review**: none\n"
	carried := closeoutCarriedFields(block)
	if got := carried["Files affected"]; got != "a.md" {
		t.Errorf("Files affected = %q", got)
	}
	if got := carried["Signal review"]; got != "none" {
		t.Errorf("Signal review = %q", got)
	}
	if _, ok := carried["Phase 1 — CONFIRMED."]; ok {
		t.Error("a verdict line was carried as a human field")
	}
}

// TestCloseoutRemainingSlotsNamesTheMarkers is the report `close` now prints:
// which of the five human slots still carry their marker.
func TestCloseoutRemainingSlotsNamesTheMarkers(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join(dir, "closeout.md")
	rendered := closeoutFactsRender(closeoutFacts{Slug: "s", PlanRef: "plan/x.md"}, "2026-01-01", nil)
	writeTestFile(t, path, rendered)

	remaining := closeoutRemainingSlots(path, "s")
	if len(remaining) != len(closeoutHumanFields) {
		t.Fatalf("remaining = %v, want all %d", remaining, len(closeoutHumanFields))
	}
	for _, fld := range closeoutHumanFields {
		if !strings.Contains(strings.Join(remaining, ","), fld.Label) {
			t.Errorf("slot %q missing from the report", fld.Label)
		}
	}

	filled := closeoutFactsRender(closeoutFacts{Slug: "s", PlanRef: "plan/x.md"}, "2026-01-01", map[string]string{"Files affected": "a.md"})
	writeTestFile(t, path, filled)
	for _, label := range closeoutRemainingSlots(path, "s") {
		if label == "Files affected" {
			t.Error("a filled slot was reported as remaining")
		}
	}
}
