package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const unitsArtifact = `---
source: test
---

# Chapter 1. One

## Alpha

one

## Beta

` + "```bash" + `
# not a heading
` + "```" + `
`

func writeUnitsArtifact(t *testing.T) {
	t.Helper()
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", unitsArtifact)
}

func TestWikiUnitsCountsAndSkipsFences(t *testing.T) {
	setupContextProject(t)
	writeUnitsArtifact(t)

	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/book-abcdef123456.md", "--format", "json")
	var report wikiUnitsReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("units json: %v\n%s", err, out)
	}
	if report.Stats.RawHeadings != 4 {
		t.Errorf("raw headings = %d, want 4 (the fenced comment included)", report.Stats.RawHeadings)
	}
	if report.Stats.OutsideFences != 3 {
		t.Errorf("outside fences = %d, want 3", report.Stats.OutsideFences)
	}
	if report.Stats.Units != 2 {
		t.Errorf("units = %d, want 2", report.Stats.Units)
	}
	if len(report.Units) != 3 { // container + two sections
		t.Errorf("rows = %d, want 3", len(report.Units))
	}
}

func TestWikiUnitsRefusesOutsidePath(t *testing.T) {
	setupContextProject(t)
	if _, err := resolveCorpusFile("/etc/hosts"); err == nil {
		t.Fatalf("resolving a path outside context/ must fail")
	}
}

func TestWikiUnitsContainerRegex(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/fr-abcdef123456.md", "---\nsource: t\n---\n\n# Chapitre 1. Un\n\n## Section\n\ntexte\n")
	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/fr-abcdef123456.md", "--containers", "^Chapitre", "--format", "json")
	var report wikiUnitsReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("units json: %v\n%s", err, out)
	}
	if report.Stats.Units != 1 {
		t.Errorf("units = %d, want 1 with a custom container regex", report.Stats.Units)
	}
}

func TestResolveCorpusFilePrefersContextRelative(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/x-abcdef123456.md", "---\nsource: t\n---\n\n# A\n")
	path, err := resolveCorpusFile("refs/converted/x-abcdef123456.md")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.HasPrefix(path, "context/") {
		t.Errorf("path = %q, want a context-relative path", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("resolved path does not exist: %v", err)
	}
}
