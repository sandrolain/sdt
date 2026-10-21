package templates

import (
	"regexp"
	"strings"
)

// The distribution gate.
//
// SDT writes its instruction set into other projects. A module may describe a
// rule, a behaviour or an example, but it may not name the repository SDT was
// developed in: its packages, its source paths, its documents, its toolchain,
// its host or its model ids. `TestGeneratedSetIsViewerFree` covered one word;
// this table covers the class.
//
// A line that genuinely needs one of these words — "vuln scans when declared:
// govulncheck, trivy" is an example of two ecosystems, not a claim about this
// one — carries `<!-- leak-ok: <reason> -->`. The marker exempts the line it
// sits on, and the following line when it stands alone, so the exemption is
// visible where the text is written and the reason travels with it.

// leakPattern is one forbidden reference.
type leakPattern struct {
	Name   string
	Re     *regexp.Regexp
	Reason string
}

// leakAllowMarker is the comment that exempts a line from the table.
const leakAllowMarker = "leak-ok:"

// leakPatterns is the table. Each row is a word-boundary or path-anchored
// pattern: a bare `internal/` is not a leak (a Go recipient has one), while
// `internal/ctxquery` is.
var leakPatterns = []leakPattern{
	{
		Name:   "viewer",
		Re:     regexp.MustCompile(`\b(viewer|sdtviewer)\b`),
		Reason: "the viewer is this repository's product, not the recipient's",
	},
	{
		Name:   "repo-package",
		Re:     regexp.MustCompile(`\b(ctxquery|bleve|contextwiki|acpbridge|mdindex)\b`),
		Reason: "an SDT Go package: the recipient has no such import path",
	},
	{
		Name:   "repo-internal-path",
		Re:     regexp.MustCompile(`\binternal/(ctxquery|contextwiki|acpbridge|mdindex|templates|search)\b`),
		Reason: "an SDT source path",
	},
	{
		Name:   "viewer-path",
		Re:     regexp.MustCompile(`\bweb/src\b|\bviewer/dist\b`),
		Reason: "a path under this repository's viewer",
	},
	{
		Name:   "repo-dir",
		Re:     regexp.MustCompile(`(^|[\s"'(=])\./(cli|viewer)/`),
		Reason: "a build step written against this repository's layout",
	},
	{
		Name:   "lint-stack",
		Re:     regexp.MustCompile(`\b(golangci-lint|gofmt|govulncheck)\b`),
		Reason: "this repository's lint stack, presented as the rule",
	},
	{
		Name:   "host",
		Re:     regexp.MustCompile(`\b(opencode|big-pickle|agent-browser|playwright-cli)\b`),
		Reason: "a development host, model codename or tool pin",
	},
	{
		Name:   "test-stack",
		Re:     regexp.MustCompile(`\b(jsdom|vitest)\b`),
		Reason: "this repository's test stack, presented as the rule",
	},
	{
		Name:   "log-library",
		Re:     regexp.MustCompile(`\blmittmann\b`),
		Reason: "the library SDT renders its logs with",
	},
	{
		Name:   "project-id",
		Re:     regexp.MustCompile(`\bsdt_[0-9a-f]{6,}\b`),
		Reason: "a concrete project id where the contract needs a placeholder",
	},
	{
		Name:   "decision-number",
		Re:     regexp.MustCompile(`\bdecision[ \t]+[0-9]{4}\b`),
		Reason: "a numbered decision from a corpus the recipient does not have",
	},
	{
		Name:   "decision-date",
		Re:     regexp.MustCompile(`\bdecision[ \t]+20[0-9]{2}-[0-9]{2}-[0-9]{2}\b`),
		Reason: "a dated decision from this repository's history",
	},
	{
		Name:   "plan-id",
		Re:     regexp.MustCompile(`\bplan[ \t]+20[0-9]{6}-[0-9]{6}\b`),
		Reason: "a concrete plan id: the recipient has no such document",
	},
}

// LeakHit is one forbidden reference found in a text.
type LeakHit struct {
	Pattern string
	Line    int
	Text    string
	Reason  string
}

// ScanLeaks returns every forbidden reference in text, one hit per matching
// line and pattern. A line carrying the allow marker is skipped, and so is the
// line after a marker that stands alone.
func ScanLeaks(text string) []LeakHit {
	var hits []LeakHit
	lines := strings.Split(text, "\n")
	skipNext := false
	for i, line := range lines {
		if skipNext {
			skipNext = false
			continue
		}
		if idx := strings.Index(line, leakAllowMarker); idx >= 0 {
			// A marker alone on its line exempts the following line; a marker
			// sharing a line with the text exempts only that line. Either way
			// the marker's own line is skipped.
			skipNext = markerAlone(line[:idx])
			continue
		}
		for _, p := range leakPatterns {
			if p.Re.MatchString(line) {
				hits = append(hits, LeakHit{Pattern: p.Name, Line: i + 1, Text: strings.TrimSpace(line), Reason: p.Reason})
			}
		}
	}
	return hits
}

// markerAlone reports whether the marker opens its line — nothing but an HTML
// comment opener before it — so the exemption applies to the next line. The
// reason after the marker never decides this: it is always there.
func markerAlone(before string) bool {
	before = strings.ReplaceAll(before, "<!--", "")
	return strings.TrimSpace(before) == ""
}

// LeakPatternNames returns the table's pattern names in declared order, so a
// caller can prove every row has a negative fixture.
func LeakPatternNames() []string {
	names := make([]string, 0, len(leakPatterns))
	for _, p := range leakPatterns {
		names = append(names, p.Name)
	}
	return names
}

// LeakMarker is the allow marker, exported so a caller can build a fixture that
// exercises it.
const LeakMarker = leakAllowMarker
