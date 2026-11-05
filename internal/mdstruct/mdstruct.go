// Package mdstruct provides pure-Go helpers to split Markdown documents into
// fence-aware sections and to compact a document by dropping whole sections
// while returning a manifest of what was removed.
//
// It is the context-safety base for agent context assembly and for the planned
// hybrid search (Wave 2): the search index unit is a section with a stable id,
// and compaction must never cut through a code fence or silently drop content.
package mdstruct

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Section is one heading and its body, up to the next heading at any level.
// The boundary is flat on purpose: a subsection is its own section, so
// `read --section` addresses the smallest unit and replacing a section cannot
// silently delete its subsections.
type Section struct {
	// ID is a stable identifier: the heading anchor followed by an ordinal when
	// the anchor repeats. Empty headings use the anchor "section".
	ID string
	// Heading is the raw heading text (without the leading '#'), "" for the
	// preamble section before the first heading.
	Heading string
	// Level is the heading depth (1-6), 0 for the preamble.
	Level int
	// Body is the section content including the heading line; it ends with "\n"
	// unless the section is the last and the source had no trailing newline.
	Body string
	// StartLine and EndLine are 1-based line ranges in the source.
	StartLine int
	EndLine   int
}

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
var fenceRe = regexp.MustCompile("^(\\s*)(```+|~~~+)")

// OpenFence is a fenced code block left unterminated at the end of a document.
// Line is 1-based and Marker is the opening run, so an error can name both.
type OpenFence struct {
	Line   int
	Marker string
}

// fenceRun returns the fence marker at the start of line — a run of 3+ backticks
// or 3+ tildes after optional whitespace — and whether the line carries one.
func fenceRun(line string) (char byte, length int, ok bool) {
	m := fenceRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	return m[2][0], len(m[2]), true
}

// ScanFences marks every line that is a fence delimiter or lies inside a fenced
// code block, and reports a fence still open at the end. The length rule is
// CommonMark's: a fence closes only on a run of the same character at least as
// long as the opening one, so a three-backtick line inside a four-backtick block
// is content, not a close. An unterminated fence runs to end of file.
func ScanFences(lines []string) ([]bool, *OpenFence) {
	inside := make([]bool, len(lines))
	var openChar byte
	openLen, openLine := 0, 0
	for i, line := range lines {
		char, length, ok := fenceRun(line)
		switch {
		case openLen == 0:
			if ok {
				openChar, openLen, openLine = char, length, i+1
				inside[i] = true
			}
		case ok && char == openChar && length >= openLen:
			inside[i] = true
			openLen = 0
		default:
			inside[i] = true
		}
	}
	if openLen == 0 {
		return inside, nil
	}
	return inside, &OpenFence{Line: openLine, Marker: strings.Repeat(string(openChar), openLen)}
}

// UnclosedFence reports the fence left open at the end of doc, or nil when every
// fence closes. A caller that resolves sections asks first: while a fence is
// open, no boundary below it can be trusted.
func UnclosedFence(doc string) *OpenFence {
	_, open := ScanFences(splitDocLines(doc))
	return open
}

// splitDocLines splits doc and drops the single empty element a terminating
// newline produces, so line numbers index the result directly.
func splitDocLines(doc string) []string {
	lines := strings.Split(doc, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// SplitSections splits a Markdown document into sections by ATX headings while
// ignoring heading-like lines inside fenced code blocks. Any content before the
// first heading becomes a preamble section with an empty Heading.
func SplitSections(doc string) []Section {
	lines := splitDocLines(doc)
	inside, _ := ScanFences(lines)

	type heading struct {
		index int
		level int
		text  string
	}
	var headings []heading
	for i, line := range lines {
		if inside[i] {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			headings = append(headings, heading{index: i, level: len(m[1]), text: m[2]})
		}
	}

	var sections []Section
	ordinals := map[string]int{}
	newSection := func(start, end int, h heading, level int) Section {
		anchor := HeadingAnchor(h.text)
		if anchor == "" {
			anchor = "section"
		}
		ordinals[anchor]++
		id := anchor
		if ordinals[anchor] > 1 {
			id = fmt.Sprintf("%s-%d", anchor, ordinals[anchor])
		}
		return Section{
			ID:        id,
			Heading:   h.text,
			Level:     level,
			Body:      strings.Join(lines[start:end+1], "\n") + "\n",
			StartLine: start + 1,
			EndLine:   end + 1,
		}
	}

	if len(headings) == 0 {
		if len(lines) == 0 {
			return nil
		}
		return []Section{newSection(0, len(lines)-1, heading{}, 0)}
	}
	// Preamble before the first heading.
	if headings[0].index > 0 {
		sections = append(sections, newSection(0, headings[0].index-1, heading{}, 0))
	}
	for i, h := range headings {
		end := len(lines) - 1
		if i+1 < len(headings) {
			end = headings[i+1].index - 1
		}
		sections = append(sections, newSection(h.index, end, h, h.level))
	}
	return sections
}

// HeadingAnchor converts heading text into a stable, lowercase, hyphenated
// anchor usable as a section id (e.g. "## Wave 1 — start here" ->
// "wave-1-start-here").
func HeadingAnchor(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastDash := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '/' || r == '.':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// DroppedSection describes a section removed by Compact.
type DroppedSection struct {
	ID      string `json:"id" yaml:"id"`
	Heading string `json:"heading" yaml:"heading"`
	Lines   int    `json:"lines" yaml:"lines"`
	Hash    string `json:"hash" yaml:"hash"`
}

// Manifest records what Compact removed, so the caller can report or restore
// the dropped content instead of losing it silently.
type Manifest struct {
	KeptSections    []string         `json:"kept_sections" yaml:"kept_sections"`
	DroppedSections []DroppedSection `json:"dropped_sections" yaml:"dropped_sections"`
	OriginalLines   int              `json:"original_lines" yaml:"original_lines"`
	CompactedLines  int              `json:"compacted_lines" yaml:"compacted_lines"`
}

// Compact drops the sections selected by drop (called with each Section) and
// returns the compacted document plus the manifest of what was removed. Dropped
// sections are whole sections: a section is never split. The preamble (empty
// Heading, Level 0) is always kept.
func Compact(doc string, drop func(Section) bool) (string, Manifest) {
	sections := SplitSections(doc)
	m := Manifest{OriginalLines: countLines(doc)}
	var kept []string
	for _, s := range sections {
		if s.Level != 0 && drop != nil && drop(s) {
			m.DroppedSections = append(m.DroppedSections, DroppedSection{
				ID:      s.ID,
				Heading: s.Heading,
				Lines:   s.EndLine - s.StartLine + 1,
				Hash:    shortHash(s.Body),
			})
			continue
		}
		m.KeptSections = append(m.KeptSections, s.ID)
		kept = append(kept, strings.TrimRight(s.Body, "\n"))
	}
	out := strings.Join(kept, "\n\n")
	if out != "" {
		out += "\n"
	}
	m.CompactedLines = countLines(out)
	return out, m
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(strings.TrimRight(s, "\n"), "\n") + 1
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
