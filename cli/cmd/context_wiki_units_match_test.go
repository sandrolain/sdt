package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

const unitsMatchArtifact = `---
source: test
---

# Chapter 1. Foundations

## Scalability and growth

Scalability is a system's capability to handle growth in a dimension of its
operation, such as request volume or data volume.

## Caching repeated reads

A cache serves repeated reads from memory instead of the database, which lowers
database load as the hit ratio rises.

## Byzantine agreement

Participants that lie are excluded from the practical consensus problem because
systems run behind protected networks.
`

func writeUnitsMatchArtifact(t *testing.T) {
	t.Helper()
	writeCtxDoc(t, "context/refs/converted/match-abcdef123456.md", unitsMatchArtifact)
}

func writeUnitsMatchPages(t *testing.T) {
	t.Helper()
	writeCtxDoc(t, "context/wiki/architecture/scalability.md",
		"---\nkind: wiki\ntitle: scalability\nsummary: growth in a dimension of operation\nstatus: draft\n---\n\n## Summary\n\nscalability is growth in a dimension of operation.\n")
	writeCtxDoc(t, "context/wiki/architecture/caching.md",
		"---\nkind: wiki\ntitle: caching\nsummary: serving repeated reads from memory instead of the database\nstatus: draft\n---\n\n## Summary\n\nA cache serves repeated reads from memory.\n")
}

func matchReport(t *testing.T, out []byte) wikiUnitsMatchReport {
	t.Helper()
	var report wikiUnitsMatchReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("units --match json: %v\n%s", err, out)
	}
	return report
}

func matchRow(t *testing.T, report wikiUnitsMatchReport, title string) wikiUnitRow {
	t.Helper()
	for _, row := range report.Units {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("unit %q not in the report", title)
	return wikiUnitRow{}
}

func TestWikiUnitsMatchRanksTheRightPage(t *testing.T) {
	setupContextProject(t)
	writeUnitsMatchArtifact(t)
	writeUnitsMatchPages(t)

	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/match-abcdef123456.md",
		"--match", "wiki/architecture", "--limit", "1", "--format", "json")
	report := matchReport(t, out)
	if report.Match != "wiki/architecture" {
		t.Errorf("match dir = %q", report.Match)
	}

	growth := matchRow(t, report, "Scalability and growth")
	if len(growth.Matches) != 1 || growth.Matches[0].Slug != "architecture/scalability" {
		t.Fatalf("scalability matches = %+v", growth.Matches)
	}
	if len(growth.Matches[0].Terms) == 0 {
		t.Errorf("no evidence terms on the scalability match")
	}

	cache := matchRow(t, report, "Caching repeated reads")
	if len(cache.Matches) != 1 || cache.Matches[0].Slug != "architecture/caching" {
		t.Fatalf("caching matches = %+v", cache.Matches)
	}
}

func TestWikiUnitsMatchReportsNoCandidate(t *testing.T) {
	setupContextProject(t)
	writeUnitsMatchArtifact(t)
	writeUnitsMatchPages(t)

	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/match-abcdef123456.md",
		"--match", "wiki/architecture", "--limit", "3")
	text := string(out)
	if !strings.Contains(text, "Byzantine agreement") || !strings.Contains(text, "(no candidate)") {
		t.Fatalf("text output does not report the empty candidate set:\n%s", text)
	}
}

func TestWikiUnitsMatchCapsAtLimit(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/match-abcdef123456.md",
		"---\nsource: t\n---\n\n# Chapter 1\n\n## Caching and scalability\n\nA cache serves repeated reads from memory, and scalability is growth in a dimension of operation.\n")
	writeUnitsMatchPages(t)

	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/match-abcdef123456.md",
		"--match", "wiki/architecture", "--limit", "1", "--format", "json")
	report := matchReport(t, out)
	row := matchRow(t, report, "Caching and scalability")
	if len(row.Matches) != 1 || !row.Capped {
		t.Fatalf("matches = %+v capped=%v, want one match and the cap reported", row.Matches, row.Capped)
	}
}

func TestWikiUnitsMatchWithoutProjectRootFails(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/converted/match-abcdef123456.md", unitsMatchArtifact)
	writeTestFile(t, "context/wiki/architecture/scalability.md",
		"---\nkind: wiki\ntitle: scalability\nsummary: growth\nstatus: draft\n---\n\nbody\n")
	logs := captureCmdErr(t)

	shouldExitWithCode(t, 1, func() string {
		execute(t, contextWikiUnitsCmd, nil, "refs/converted/match-abcdef123456.md", "--match", "wiki/architecture")
		return ""
	})
	if !strings.Contains(logs.String(), "--match needs a project root") {
		t.Errorf("error does not name the missing project root: %s", logs.String())
	}
}

func TestWikiUnitsWithoutMatchKeepsTheReportShape(t *testing.T) {
	setupContextProject(t)
	writeUnitsMatchArtifact(t)

	out := execute(t, contextWikiUnitsCmd, nil, "refs/converted/match-abcdef123456.md", "--format", "json")
	if strings.Contains(string(out), `"match"`) || strings.Contains(string(out), `"matches"`) {
		t.Fatalf("unflagged report carries match fields: %s", out)
	}
}
