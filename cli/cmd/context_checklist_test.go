package cmd

import (
	"strings"
	"testing"
)

func TestParseChecklistID(t *testing.T) {
	for _, ok := range []string{"c3", "3", "C3", " c7 "} {
		if _, valid := parseChecklistID(ok); !valid {
			t.Errorf("parseChecklistID(%q) should be valid", ok)
		}
	}
	for _, bad := range []string{"", "c", "0", "-1", "x", "c1a"} {
		if _, valid := parseChecklistID(bad); valid {
			t.Errorf("parseChecklistID(%q) should be invalid", bad)
		}
	}
}

func TestParseChecklistItemsStripsAnchor(t *testing.T) {
	content := "- [ ] step one <!-- c1 -->\n- [x] step two <!-- c9 -->\n- [~] no anchor\n"
	items := parseChecklistItems(content)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Text != "step one" || items[0].ID != "c1" {
		t.Errorf("item 0 = %q/%q", items[0].Text, items[0].ID)
	}
	if items[1].Status != taskStatusDone || items[1].ID != "c9" {
		t.Errorf("item 1 = %q/%q", items[1].Status, items[1].ID)
	}
	if items[2].ID != "" || items[2].Text != "no anchor" {
		t.Errorf("item 2 = %q/%q", items[2].Text, items[2].ID)
	}
}

func TestStampChecklistIDs(t *testing.T) {
	content := "- [ ] one\n- [x] two <!-- c2 -->\n- [ ] three\n"
	got, changed := stampChecklistIDs(content)
	if !changed {
		t.Fatal("expected change")
	}
	want := "- [ ] one <!-- c3 -->\n- [x] two <!-- c2 -->\n- [ ] three <!-- c4 -->\n"
	if got != want {
		t.Fatalf("stamp = %q, want %q", got, want)
	}
	again, changed := stampChecklistIDs(got)
	if changed || again != got {
		t.Fatalf("second stamp must be a no-op, changed=%v", changed)
	}
}

func TestChecklistIDAddressIsReorderSafe(t *testing.T) {
	content := "- [ ] alpha <!-- c1 -->\n- [ ] beta <!-- c2 -->\n- [ ] gamma <!-- c3 -->\n"
	reordered := "- [ ] gamma <!-- c3 -->\n- [ ] alpha <!-- c1 -->\n- [ ] beta <!-- c2 -->\n"
	got, err := updateChecklistItem(reordered, "c1", taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- [x] alpha <!-- c1 -->") {
		t.Fatalf("anchor must follow the item, not its position:\n%s", got)
	}
	_ = content
}

func TestUpdateChecklistItemByAnchorAndOrdinal(t *testing.T) {
	content := "- [ ] one\n- [ ] two\n"

	byOrdinal, err := updateChecklistItem(content, "2", taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(byOrdinal, "- [x] two <!-- c2 -->") {
		t.Fatalf("ordinal fallback failed:\n%s", byOrdinal)
	}

	byAnchor, err := updateChecklistItem(content, "c1", taskStatusBlock, "waiting")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(byAnchor, "- [!] one (blocked: waiting) <!-- c1 -->") {
		t.Fatalf("anchor update failed:\n%s", byAnchor)
	}

	if _, err := updateChecklistItem(content, "c9", taskStatusDone, ""); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestParseChecklistItemsMultiLineAnchor(t *testing.T) {
	content := "- [ ] first line\n      continuation <!-- c7 -->\n- [x] second\n"
	items := parseChecklistItems(content)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "c7" {
		t.Errorf("item 0 id = %q, want c7 (anchor on the continuation line)", items[0].ID)
	}
	if items[0].Text != "first line" {
		t.Errorf("item 0 text = %q, want the checklist line body", items[0].Text)
	}
	if items[1].ID != "" {
		t.Errorf("item 1 id = %q, want empty", items[1].ID)
	}
}

// TestStampChecklistIDsNoDoubleAnchor is the regression for the observed
// damage: an item whose anchor sits on its last continuation line must not gain
// a second, freshly stamped anchor on the checklist line.
func TestStampChecklistIDsNoDoubleAnchor(t *testing.T) {
	content := "## Phase 1\n" +
		"- [ ] first phase item\n" +
		"      continuation with the authored anchor <!-- c1 -->\n" +
		"\n" +
		"## Phase 2\n" +
		"- [ ] second phase item\n" +
		"      continuation with the authored anchor <!-- c1 -->\n"
	got, changed := stampChecklistIDs(content)
	if changed || got != content {
		t.Fatalf("stamping must be a no-op on an anchored item, changed=%v:\n%s", changed, got)
	}
	if strings.Count(got, "<!-- c1 -->") != 2 {
		t.Fatalf("expected exactly the two authored anchors, got:\n%s", got)
	}
}

func TestStampChecklistIDsSkipsFencedCode(t *testing.T) {
	content := "```\n- [ ] sample in a snippet\n```\n- [ ] real item\n"
	got, changed := stampChecklistIDs(content)
	if !changed {
		t.Fatal("the real item should be stamped")
	}
	if !strings.Contains(got, "- [ ] sample in a snippet\n") {
		t.Fatalf("a fenced sample must not be stamped:\n%s", got)
	}
	if !strings.Contains(got, "- [ ] real item <!-- c1 -->") {
		t.Fatalf("the real item should carry c1:\n%s", got)
	}
}

// TestUpdateChecklistItemNormalizesAnchor: a write moves the item's anchor onto
// the checklist line and removes it from the continuation line.
func TestUpdateChecklistItemNormalizesAnchor(t *testing.T) {
	content := "- [ ] first phase item\n      continuation with the authored anchor <!-- c1 -->\n"
	got, err := updateChecklistItem(content, "c1", taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "- [x] first phase item <!-- c1 -->\n      continuation with the authored anchor\n"
	if got != want {
		t.Fatalf("normalize = %q, want %q", got, want)
	}
	// A second write is stable (idempotent) and still resolves by the same id.
	again, err := updateChecklistItem(got, "c1", taskStatusTodo, "")
	if err != nil {
		t.Fatal(err)
	}
	if again != "- [ ] first phase item <!-- c1 -->\n      continuation with the authored anchor\n" {
		t.Fatalf("second write not stable:\n%s", again)
	}
}

func TestUpdateChecklistItemAmbiguousIDRefused(t *testing.T) {
	content := "## Phase 1\n- [ ] a1 <!-- c1 -->\n- [ ] a2 <!-- c2 -->\n\n## Phase 2\n- [ ] b1 <!-- c1 -->\n- [ ] b2 <!-- c2 -->\n"
	got, err := updateChecklistItem(content, "c1", taskStatusDone, "")
	if err == nil {
		t.Fatalf("an ambiguous id must be refused, not resolved to the first match:\n%s", got)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error should name the ambiguity, got %v", err)
	}
	// The ordinal fallback still targets one specific item (here the 3rd = b1).
	byOrdinal, err := updateChecklistItem(content, "3", taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(byOrdinal, "- [x] b1 <!-- c1 -->") {
		t.Fatalf("ordinal fallback failed:\n%s", byOrdinal)
	}
}

func TestChecklistIDGlobalNumberingResolves(t *testing.T) {
	content := "## Phase 1\n- [ ] a1 <!-- c1 -->\n\n## Phase 2\n- [ ] b1 <!-- c2 -->\n"
	got, err := updateChecklistItem(content, "c2", taskStatusDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- [x] b1 <!-- c2 -->") {
		t.Fatalf("a globally unique id must hit the right item:\n%s", got)
	}
}

func TestRepairChecklistIDsDropsStrayAndRenumbers(t *testing.T) {
	content := "## Phase 1\n- [ ] a1 <!-- c1 -->\n      stray <!-- c1 -->\n\n## Phase 2\n- [ ] b1 <!-- c1 -->\n"
	got, changed := repairChecklistIDs(content)
	if !changed {
		t.Fatal("expected a repair")
	}
	want := "## Phase 1\n- [ ] a1 <!-- c1 -->\n      stray\n\n## Phase 2\n- [ ] b1 <!-- c2 -->\n"
	if got != want {
		t.Fatalf("repair = %q, want %q", got, want)
	}
	again, changed := repairChecklistIDs(got)
	if changed || again != got {
		t.Fatalf("repair must be idempotent, changed=%v:\n%s", changed, again)
	}
}

func TestRepairChecklistIDsAdoptsContinuationAnchor(t *testing.T) {
	content := "## Phase 1\n- [ ] a1\n      text <!-- c3 -->\n"
	got, changed := repairChecklistIDs(content)
	if !changed {
		t.Fatal("expected a repair")
	}
	want := "## Phase 1\n- [ ] a1 <!-- c3 -->\n      text\n"
	if got != want {
		t.Fatalf("repair = %q, want %q", got, want)
	}
}

func TestRepairChecklistIDsPreservesReviewBlock(t *testing.T) {
	content := "## Phase 1\n- [ ] a1 <!-- c1 -->\n- [ ] b1 <!-- c1 -->\n\n## Review\n\nVerify-step protocol.\n- **CONFIRMED** — done.\n"
	got, changed := repairChecklistIDs(content)
	if !changed {
		t.Fatal("the duplicate should be renumbered")
	}
	if !strings.Contains(got, "## Review\n\nVerify-step protocol.\n- **CONFIRMED** — done.\n") {
		t.Fatalf("Review block must be untouched:\n%s", got)
	}
	if !strings.Contains(got, "- [ ] b1 <!-- c2 -->") {
		t.Fatalf("the duplicate item should be renumbered to c2:\n%s", got)
	}
}
