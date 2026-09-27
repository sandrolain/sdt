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
