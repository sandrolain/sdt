package distill

import (
	"errors"
	"os"
	"testing"
)

const sample = `---
source: example
---

# Book Title

# Preface

## Why Read This

# Part I. Mechanics

# Chapter 1. First

## Section A

Line one.

## Section B

Line two.

` + "```bash" + `
# not a heading
` + "```" + `

# Index

## Entries
`

func TestParseSkipsFencedHeadingsAndCountsContainers(t *testing.T) {
	units, stats := Parse(sample, Options{})
	if stats.RawHeadings != 10 {
		t.Errorf("raw headings = %d, want 10 (the fenced `# not a heading` included)", stats.RawHeadings)
	}
	if stats.OutsideFences != 9 {
		t.Errorf("outside fences = %d, want 9", stats.OutsideFences)
	}
	if stats.Units != 5 {
		t.Errorf("units = %d, want 5", stats.Units)
	}
	if stats.Headings23 != 4 {
		t.Errorf("headings 2-3 = %d, want 4", stats.Headings23)
	}
	kinds := map[string]int{}
	for _, u := range units {
		kinds[u.Kind]++
	}
	if kinds[UnitContainer] != 4 || kinds[UnitSection] != 2 || kinds[UnitFrontBack] != 3 {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestParseChapterAndFrontBackPropagation(t *testing.T) {
	units, _ := Parse(sample, Options{})
	var sectionB Unit
	for _, u := range units {
		if u.Title == "Section B" {
			sectionB = u
		}
		if u.Title == "Entries" && u.Kind != UnitFrontBack {
			t.Errorf("a section inside the Index must be front-back, got %q", u.Kind)
		}
	}
	if sectionB.Chapter != "Chapter 1. First" {
		t.Errorf("Section B chapter = %q", sectionB.Chapter)
	}
	if sectionB.Line == 0 || sectionB.End < sectionB.Line {
		t.Errorf("span = %d-%d", sectionB.Line, sectionB.End)
	}
}

func TestClassifyOKPartGap(t *testing.T) {
	units, _ := Parse(sample, Options{})
	var a, b Unit
	for _, u := range units {
		switch u.Title {
		case "Section A":
			a = u
		case "Section B":
			b = u
		}
	}
	ranges := []Range{
		{Start: a.Line, End: a.End},  // A fully cited
		{Start: b.Line, End: b.Line}, // B partially cited (heading only)
	}
	out := Classify(units, ranges, sample)
	states := map[string]string{}
	for _, u := range out {
		states[u.Title] = u.State
	}
	if states["Section A"] != StateOK {
		t.Errorf("A = %q, want OK", states["Section A"])
	}
	if states["Section B"] != StatePart {
		t.Errorf("B = %q, want PART", states["Section B"])
	}
	if states["Why Read This"] != StateGap {
		t.Errorf("uncited = %q, want GAP", states["Why Read This"])
	}
}

func TestCount(t *testing.T) {
	units := []Unit{
		{Kind: UnitSection, State: StateOK},
		{Kind: UnitSection, State: StatePart},
		{Kind: UnitSection, State: StateGap, Disposition: DispositionNotConceptual},
		{Kind: UnitSection, State: StateGap},
		{Kind: UnitContainer},
	}
	c := Count(units)
	if c.Units != 4 || c.OK != 1 || c.Part != 1 || c.NoOp != 1 || c.Gap != 1 {
		t.Errorf("counts = %+v", c)
	}
	if c.OK+c.Part+c.Gap+c.NoOp != c.Units {
		t.Errorf("identity does not close: %+v", c)
	}
}

func TestMergeRanges(t *testing.T) {
	got := MergeRanges([]Range{{5, 8, "a"}, {1, 2, "a"}, {3, 4, "a"}, {10, 10, "a"}})
	if len(got) != 2 || got[0].Start != 1 || got[0].End != 8 || got[1].Start != 10 {
		t.Errorf("merged = %+v", got)
	}
}

func TestParseCitations(t *testing.T) {
	body := "Claim (refs/converted/x.md@abcdef12:10-12) and (refs/converted/x.md@abcdef12:20).\n" +
		"Example `refs/converted/x.md@abcdef12:30` is code.\n" +
		"```\nrefs/converted/x.md@abcdef12:40\n```\n"
	cites := ParseCitations(body)
	if len(cites) != 2 {
		t.Fatalf("citations = %+v", cites)
	}
	if cites[0].Line != 10 || cites[0].End != 12 {
		t.Errorf("range = %+v", cites[0])
	}
	if cites[1].Line != 20 || cites[1].Whole {
		t.Errorf("single = %+v", cites[1])
	}
}

func TestBodyLinesExcludesFrontmatter(t *testing.T) {
	if got := BodyLines("---\na: b\n---\n\none\ntwo\n"); got != 2 {
		t.Errorf("body lines = %d, want 2", got)
	}
}

func TestParseRealArtifactCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the two book artifacts")
	}
	paths := map[string]struct {
		outside int
		units   int
	}{
		"../../context/refs/converted/buildingevolutionaryarchitectures2ndedition-c5ff5ad972ff.md": {240, 223},
		"../../context/refs/converted/softwarearchitecture-thehardparts-d6c2b426fa8c.md":           {268, 246},
	}
	for path, want := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			// context/ is gitignored (the repo tracks code only), so a fresh
			// checkout — CI included — has no captured artifact to parse.
			t.Skipf("%s absent (context/ is gitignored); the corpus guard is local-only", path)
		}
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, stats := Parse(string(data), Options{})
		if stats.OutsideFences != want.outside {
			t.Errorf("%s: outside fences = %d, want %d", path, stats.OutsideFences, want.outside)
		}
		if stats.Units != want.units {
			t.Errorf("%s: units = %d, want %d", path, stats.Units, want.units)
		}
	}
}
