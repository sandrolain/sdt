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

// Section is one heading and its body (up to the next heading of the same or a
// higher level).
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

// SplitSections splits a Markdown document into sections by ATX headings while
// ignoring heading-like lines inside fenced code blocks. Any content before the
// first heading becomes a preamble section with an empty Heading.
func SplitSections(doc string) []Section {
	lines := strings.Split(doc, "\n")
	// Drop the single trailing element produced by a terminating newline so line
	// ranges stay accurate.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	type heading struct {
		index int
		level int
		text  string
	}
	var headings []heading
	fence := ""
	for i, line := range lines {
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			marker := m[2]
			switch {
			case fence == "":
				fence = marker[:1]
			case marker[0:1] == fence && len(marker) >= len(fence)*3:
				fence = ""
			}
			continue
		}
		if fence != "" {
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
