package mdstruct

import (
	"strings"
	"testing"
)

func TestSplitSectionsBasic(t *testing.T) {
	doc := "intro line\n\n## One\n\nbody one\n\n### One A\n\nbody a\n\n## Two\n\nbody two\n"
	secs := SplitSections(doc)
	if len(secs) != 4 {
		t.Fatalf("expected 4 sections, got %d: %+v", len(secs), secs)
	}
	if secs[0].Heading != "" || secs[0].Level != 0 {
		t.Errorf("expected a level-0 preamble, got %+v", secs[0])
	}
	want := []struct {
		id      string
		heading string
		level   int
	}{
		{"section", "", 0},
		{"one", "One", 2},
		{"one-a", "One A", 3},
		{"two", "Two", 2},
	}
	for i, w := range want {
		if secs[i].ID != w.id || secs[i].Heading != w.heading || secs[i].Level != w.level {
			t.Errorf("section %d = %+v, want id=%q heading=%q level=%d", i, secs[i], w.id, w.heading, w.level)
		}
	}
	if secs[1].StartLine != 3 || secs[1].EndLine != 6 {
		t.Errorf("unexpected One line range: %d-%d", secs[1].StartLine, secs[1].EndLine)
	}
}

func TestSplitSectionsIgnoresFencedHeadings(t *testing.T) {
	doc := "## Real\n\n" + "```go\n// ## Not a heading\nfunc f() {}\n```\n\n## Also real\n\nbody\n"
	secs := SplitSections(doc)
	if len(secs) != 2 {
		t.Fatalf("expected 2 sections, got %d: %+v", len(secs), secs)
	}
	if secs[0].ID != "real" || secs[1].ID != "also-real" {
		t.Errorf("unexpected ids: %q, %q", secs[0].ID, secs[1].ID)
	}
	if !strings.Contains(secs[0].Body, "## Not a heading") {
		t.Errorf("fenced heading line must stay in the body:\n%s", secs[0].Body)
	}
}

func TestSplitSectionsRepeatedHeadingOrdinal(t *testing.T) {
	doc := "## Notes\n\na\n\n## Notes\n\nb\n"
	secs := SplitSections(doc)
	if secs[0].ID != "notes" || secs[1].ID != "notes-2" {
		t.Errorf("expected notes / notes-2, got %q / %q", secs[0].ID, secs[1].ID)
	}
}

func TestSplitSectionsNoHeadingAndEmpty(t *testing.T) {
	secs := SplitSections("just a body\nsecond line")
	if len(secs) != 1 || secs[0].Level != 0 || !strings.Contains(secs[0].Body, "second line") {
		t.Errorf("unexpected single-section split: %+v", secs)
	}
	if got := SplitSections(""); len(got) != 0 {
		t.Errorf("empty document must split to no sections, got %+v", got)
	}
}

func TestSplitSectionsNoTrailingNewline(t *testing.T) {
	doc := "## A\n\nbody without trailing newline"
	secs := SplitSections(doc)
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].EndLine != 3 {
		t.Errorf("expected EndLine 3, got %d", secs[0].EndLine)
	}
	if !strings.Contains(secs[0].Body, "body without trailing newline") {
		t.Errorf("body lost:\n%q", secs[0].Body)
	}
}

func TestCompactDropsWholeSectionsWithManifest(t *testing.T) {
	doc := "intro\n\n## Keep\n\nkeep body\n\n## Drop me\n\ndropped body\n\n## Keep too\n\ntoo body\n"
	out, m := Compact(doc, func(s Section) bool { return s.ID == "drop-me" })
	if strings.Contains(out, "dropped body") {
		t.Errorf("dropped body must not survive:\n%s", out)
	}
	if !strings.Contains(out, "keep body") || !strings.Contains(out, "too body") || !strings.Contains(out, "intro") {
		t.Errorf("kept content missing:\n%s", out)
	}
	if len(m.DroppedSections) != 1 {
		t.Fatalf("expected 1 dropped section, got %+v", m.DroppedSections)
	}
	d := m.DroppedSections[0]
	if d.ID != "drop-me" || d.Heading != "Drop me" || d.Hash == "" {
		t.Errorf("unexpected dropped entry: %+v", d)
	}
	if m.OriginalLines <= m.CompactedLines {
		t.Errorf("compaction should reduce lines: %d -> %d", m.OriginalLines, m.CompactedLines)
	}
	if len(m.KeptSections) != 3 { // preamble + two kept headings
		t.Errorf("expected 3 kept sections, got %v", m.KeptSections)
	}
}

func TestCompactKeepsPreambleEvenIfSelected(t *testing.T) {
	doc := "intro\n\n## A\n\na\n"
	out, m := Compact(doc, func(s Section) bool { return true })
	if !strings.Contains(out, "intro") {
		t.Errorf("preamble must always be kept:\n%s", out)
	}
	if len(m.DroppedSections) != 1 || m.DroppedSections[0].ID != "a" {
		t.Errorf("only the heading section should drop: %+v", m.DroppedSections)
	}
}

func TestHeadingAnchor(t *testing.T) {
	cases := map[string]string{
		"Wave 1 — start here": "wave-1-start-here",
		"## Heading anchors":  "heading-anchors",
		"Multiple   spaces":   "multiple-spaces",
		"Path/like.name":      "path-like-name",
		"":                    "",
	}
	for in, want := range cases {
		if got := HeadingAnchor(in); got != want {
			t.Errorf("HeadingAnchor(%q) = %q, want %q", in, got, want)
		}
	}
}
