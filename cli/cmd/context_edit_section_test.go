package cmd

import (
	"strings"
	"testing"
)

// reviewWithGate is the shape that made `context edit --section` look like it
// spliced: an H2 carrying a table, a verdict paragraph and an H3 child. Sections
// are flat on purpose, so `## Review` ends before `### Gate`.
const reviewWithGate = `## Review

| gate | verdict |
|---|---|
| one | passed |

**Verdict.** The lane held.

### Gate

- ` + "`sdt agent gate`" + ` — passed.
`

// TestReplaceContextSectionKeepsSubsectionsFlat: replacing an H2 leaves its H3
// children in place, untouched and in the same order — the child is its own
// section, and the replacement cannot silently delete it.
func TestReplaceContextSectionKeepsSubsectionsFlat(t *testing.T) {
	text := "## Review\n\n| gate | verdict |\n|---|---|\n| two | passed |\n\n**Verdict.** Rewritten.\n"
	got, err := replaceContextSection(reviewWithGate, "Review", text)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if strings.Count(got, "### Gate") != 1 {
		t.Fatalf("the child heading appears %d times, want once:\n%s", strings.Count(got, "### Gate"), got)
	}
	if !strings.Contains(got, "- `sdt agent gate` — passed.") {
		t.Errorf("the child section was lost:\n%s", got)
	}
	if strings.Contains(got, "The lane held") {
		t.Errorf("the old body survived:\n%s", got)
	}
	if !strings.Contains(got, "Rewritten.") {
		t.Errorf("the new body is missing:\n%s", got)
	}
}

// TestReplaceContextSectionRefusesARepeatedChild: a replacement that carries the
// child's heading would duplicate the surviving child — the silent corruption
// the question recorded. The edit is refused and names the collision.
func TestReplaceContextSectionRefusesARepeatedChild(t *testing.T) {
	text := "## Review\n\n**Verdict.** Rewritten.\n\n### Gate\n\n- a second gate run\n"
	_, err := replaceContextSection(reviewWithGate, "Review", text)
	if err == nil {
		t.Fatal("a replacement repeating the child heading was accepted")
	}
	if !strings.Contains(err.Error(), "Gate") {
		t.Errorf("error does not name the colliding heading: %v", err)
	}
}

// TestReplaceContextSectionRefusesARepeatedNextSibling: the same collision one
// level out — the text repeats the heading of the section that follows.
func TestReplaceContextSectionRefusesARepeatedNextSibling(t *testing.T) {
	body := "## One\n\na\n\n## Two\n\nb\n"
	_, err := replaceContextSection(body, "One", "## One\n\na2\n\n## Two\n\nb2\n")
	if err == nil {
		t.Fatal("a replacement repeating the next sibling's heading was accepted")
	}
	if !strings.Contains(err.Error(), "Two") {
		t.Errorf("error does not name the colliding heading: %v", err)
	}
}

// TestReplaceContextSectionAllowsANewHeading: a heading the document does not
// carry yet is content, not a collision.
func TestReplaceContextSectionAllowsANewHeading(t *testing.T) {
	body := "## One\n\na\n\n## Two\n\nb\n"
	got, err := replaceContextSection(body, "One", "## One\n\na2\n\n### Detail\n\nnew\n")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !strings.Contains(got, "### Detail") || !strings.Contains(got, "## Two") {
		t.Errorf("unexpected result:\n%s", got)
	}
}
