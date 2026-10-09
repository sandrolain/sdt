package converter

import (
	"strings"
	"testing"
)

func TestRemoveSkipLinks(t *testing.T) {
	in := "Before [Skip to content](#main) after [Real link](https://x.test)."
	got := removeSkipLinks(in)
	if strings.Contains(got, "Skip to content") {
		t.Errorf("skip link not removed: %q", got)
	}
	if !strings.Contains(got, "[Real link](https://x.test)") {
		t.Errorf("real link removed: %q", got)
	}
}

func TestRepairMultilineLinks(t *testing.T) {
	if got := repairMultilineLinks("[x](\nhttps://example.test/a\n)"); got != "[x](https://example.test/a)" {
		t.Errorf("multiline link not repaired: %q", got)
	}
	if got := repairMultilineLinks("[y](https://e.test)"); got != "[y](https://e.test)" {
		t.Errorf("single-line link changed: %q", got)
	}
}

func TestScrubBase64Images(t *testing.T) {
	got := scrubBase64Images("A ![alt text](data:image/png;base64,AAAA) B")
	if strings.Contains(got, "base64") {
		t.Errorf("base64 image not scrubbed: %q", got)
	}
	if !strings.Contains(got, "alt text") {
		t.Errorf("alt text lost: %q", got)
	}
	if got := scrubBase64Images("![x](https://e.test/i.png)"); !strings.Contains(got, "https://e.test/i.png") {
		t.Errorf("normal image changed: %q", got)
	}
}

func TestCollapseBlankLines(t *testing.T) {
	if got := collapseBlankLines("a\n\n\n\nb"); got != "a\n\nb" {
		t.Errorf("got %q", got)
	}
}

func TestCleanupOrder(t *testing.T) {
	got := cleanup("[Skip to content](#c)\n\n\n\n![a](data:image/gif;base64,AAAA)")
	if strings.Contains(got, "Skip to content") {
		t.Errorf("skip link survived cleanup: %q", got)
	}
	if strings.Contains(got, "base64") {
		t.Errorf("base64 image survived cleanup: %q", got)
	}
}
