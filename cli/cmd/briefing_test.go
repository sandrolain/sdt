package cmd

import (
	"strings"
	"testing"
)

// TestBriefingInstructionContract is the content guard the audit found missing:
// `briefing.md` was the one module no test named at all, so its rules could be
// removed by an edit and nothing failed. It asserts the phrases that carry each
// rule — the no-op rule, the derive-don't-trust rule, the required frontmatter,
// the six ordered sections and the boundary — and refuses the wording the module
// must not regain.
func TestBriefingInstructionContract(t *testing.T) {
	body := ""
	for _, f := range instructionFiles("p", "g") {
		if f.name == "briefing.md" {
			body = f.body
		}
	}
	if body == "" {
		t.Fatal("briefing.md not in the generated instruction set")
	}
	for _, want := range []string{
		"orienting a newcomer",
		"## When this applies (no-op rule)",
		"the right home — do not write a briefing",
		"derive, don't trust",
		"paired with the **command that derives it**",
		"a stated value that could drift is a defect",
		"| `subject` | yes |",
		"| `verified` | no |",
		"## Artifacts",
		"## Derived facts",
		"## Delta",
		"A briefing orients; it does not decide",
		"Keep the six sections in the order above; add none",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("briefing contract missing %q:\n%s", want, body)
		}
	}
	// The module composes and cites; a wording that lets it generate prose it
	// cannot cite is the defect the rule exists to prevent.
	for _, forbid := range []string{
		"may summarise without citing",
		"trust the version recorded here",
	} {
		if strings.Contains(body, forbid) {
			t.Errorf("briefing contract must not regain %q", forbid)
		}
	}
}
