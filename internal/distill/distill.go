// Package distill holds the countable half of a book-scale ingestion: it
// enumerates the units of a converted artifact from its own headings and
// classifies them against the citations the destination pages carry.
//
// The package is deliberately model-free and read-only. It answers "what does
// the artifact contain, and which of it is cited?" — never "is the page good?".
// That split is why it can sit behind a CLI check (sdt context wiki units /
// coverage) instead of a per-run script.
package distill

import (
	"regexp"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/mdstruct"
)

// Unit kinds. A container is a chapter/part/back-matter heading that groups
// units; a section is a countable unit; front-back is navigation or narrative
// front/back matter, still enumerated and dispositioned but never owed content.
const (
	UnitContainer = "container"
	UnitSection   = "section"
	UnitFrontBack = "front-back"
)

// Coverage states, the doctrine's three classifications.
const (
	StateOK   = "OK"
	StatePart = "PART"
	StateGap  = "GAP"
)

// Dispositions for a non-OK unit.
const (
	DispositionGap           = "gap"
	DispositionNoOp          = "no-op"
	DispositionNotConceptual = "not-conceptual"
)

// Unit is one heading span of the artifact.
type Unit struct {
	// Line and End are 1-based inclusive line numbers in the artifact as read
	// (frontmatter included), so a claim's `:n-m` citation maps directly.
	Line  int    `json:"line" yaml:"line"`
	End   int    `json:"end" yaml:"end"`
	Level int    `json:"level" yaml:"level"`
	Kind  string `json:"kind" yaml:"kind"`
	Title string `json:"title" yaml:"title"`
	// State and Disposition are filled by Classify.
	State       string `json:"state,omitempty" yaml:"state,omitempty"`
	Disposition string `json:"disposition,omitempty" yaml:"disposition,omitempty"`
	// Chapter is the container title this unit sits under ("" before the first
	// container).
	Chapter string `json:"chapter,omitempty" yaml:"chapter,omitempty"`
}

// Stats are the countable facts about an artifact, the numbers a receipt quotes.
type Stats struct {
	// RawHeadings counts heading-like lines including those inside code fences.
	RawHeadings int `json:"raw_headings" yaml:"raw_headings"`
	// OutsideFences counts headings outside fenced code blocks.
	OutsideFences int `json:"outside_fences" yaml:"outside_fences"`
	// Units counts non-container headings within the requested levels.
	Units int `json:"units" yaml:"units"`
	// Headings23 counts levels 2-3 outside fences, the routing test's proxy.
	Headings23 int `json:"section_headings" yaml:"section_headings"`
	// Lines counts the artifact body (frontmatter excluded).
	Lines int `json:"lines" yaml:"lines"`
	// ByLevel counts outside-fence headings per level.
	ByLevel map[int]int `json:"by_level" yaml:"by_level"`
}

// Options tune enumeration.
type Options struct {
	// MaxLevel is the deepest heading counted as a unit (default 3).
	MaxLevel int
	// Containers marks a heading that groups units rather than being one.
	Containers *regexp.Regexp
	// FrontBack marks navigation, narrative front/back matter and case studies.
	FrontBack *regexp.Regexp
}

// DefaultContainers is the corpus convention for structural containers. A
// container groups units and is not itself counted; everything else — front
// matter, appendices, acknowledgments, index entries, case studies — is a unit,
// because the doctrine enumerates those too.
var DefaultContainers = regexp.MustCompile(`^(Part\s+[IVXLC]+.*|Chapter\s+\d+.*|Index|Preface|Foreword|Acknowledgments|About the Authors|Colophon|Revision History|Dedication)$`)

// DefaultFrontBack marks front/back matter and case-study narrative.
var DefaultFrontBack = regexp.MustCompile(`^(Index\b|Preface\b|Foreword\b|Acknowledgments\b|About the Authors\b|Colophon\b|Revision History\b|Praise for\b|Dedication\b|Case Study\b|Sysops Squad Saga\b)`)

var rawHeading = regexp.MustCompile(`^#{1,6}\s`)

// Parse enumerates the units of a converted artifact. Headings inside fenced
// code blocks are never units (a shell comment is not a section); the raw count
// is still reported so the difference is visible.
func Parse(source string, opt Options) ([]Unit, Stats) {
	if opt.MaxLevel <= 0 {
		opt.MaxLevel = 3
	}
	if opt.Containers == nil {
		opt.Containers = DefaultContainers
	}
	if opt.FrontBack == nil {
		opt.FrontBack = DefaultFrontBack
	}

	stats := Stats{ByLevel: map[int]int{}}
	for _, line := range strings.Split(source, "\n") {
		if rawHeading.MatchString(line) {
			stats.RawHeadings++
		}
	}

	sections := mdstruct.SplitSections(source)
	// mdstruct ends a section at the next heading of any level (a good split
	// unit for prose). A coverage span is different: a unit owns its
	// subsections, so it runs to the next heading at its own level or higher.
	for i := range sections {
		if sections[i].Level == 0 {
			continue
		}
		end := len(strings.Split(source, "\n"))
		for j := i + 1; j < len(sections); j++ {
			if sections[j].Level > 0 && sections[j].Level <= sections[i].Level {
				end = sections[j].StartLine - 1
				break
			}
		}
		sections[i].EndLine = end
	}
	units := make([]Unit, 0, len(sections))
	chapter := ""
	frontBackChapter := false
	for _, s := range sections {
		if s.Level == 0 || s.Heading == "" {
			continue // preamble
		}
		if s.Level > opt.MaxLevel {
			continue
		}
		stats.OutsideFences++
		stats.ByLevel[s.Level]++
		if s.Level == 2 || s.Level == 3 {
			stats.Headings23++
		}
		kind := UnitSection
		switch {
		case opt.Containers.MatchString(s.Heading):
			kind = UnitContainer
		case chapter == "":
			kind = UnitFrontBack
		case frontBackChapter, opt.FrontBack.MatchString(s.Heading):
			kind = UnitFrontBack
		}
		u := Unit{
			Line:    s.StartLine,
			End:     s.EndLine,
			Level:   s.Level,
			Kind:    kind,
			Title:   s.Heading,
			Chapter: chapter,
		}
		if kind == UnitContainer {
			chapter = s.Heading
			frontBackChapter = opt.FrontBack.MatchString(s.Heading)
			u.Chapter = ""
		}
		units = append(units, u)
		if kind != UnitContainer {
			stats.Units++
		}
	}
	stats.Lines = BodyLines(source)
	return units, stats
}

// Range is a cited span, inclusive and 1-based.
type Range struct {
	Start int
	End   int
	Ref   string
}

// Classify sets State on every non-container unit from the cited ranges. A unit
// is OK when every substantive line (non-blank, outside fenced code) is cited,
// PART when some are, GAP when none are. Container units keep an empty state.
func Classify(units []Unit, ranges []Range, source string) []Unit {
	merged := MergeRanges(ranges)
	lines := strings.Split(source, "\n")
	out := make([]Unit, len(units))
	copy(out, units)
	covered := make([]bool, len(lines)+1)
	for _, r := range merged {
		for i := r.Start; i <= r.End && i < len(covered); i++ {
			if i >= 1 {
				covered[i] = true
			}
		}
	}
	for i := range out {
		u := &out[i]
		if u.Kind == UnitContainer {
			continue
		}
		substantive, cited := 0, 0
		inFence := fenceStateAt(lines, u.Line)
		for n := u.Line; n <= u.End && n <= len(lines); n++ {
			text := strings.TrimSpace(lines[n-1])
			if isFenceDelimiter(text) {
				inFence = !inFence
				continue
			}
			if inFence || text == "" {
				continue
			}
			substantive++
			if covered[n] {
				cited++
			}
		}
		switch {
		case substantive == 0:
			u.State = StateGap
		case cited == substantive:
			u.State = StateOK
		case cited > 0:
			u.State = StatePart
		default:
			u.State = StateGap
		}
	}
	return out
}

// MergeRanges sorts and merges overlapping or adjacent ranges.
func MergeRanges(ranges []Range) []Range {
	if len(ranges) == 0 {
		return nil
	}
	sorted := make([]Range, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start != sorted[j].Start {
			return sorted[i].Start < sorted[j].Start
		}
		return sorted[i].End < sorted[j].End
	})
	out := []Range{sorted[0]}
	for _, r := range sorted[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// Counts summarises a classified unit list for the coverage block.
type Counts struct {
	Units int `json:"units" yaml:"units"`
	OK    int `json:"ok" yaml:"ok"`
	Part  int `json:"part" yaml:"part"`
	Gap   int `json:"gap" yaml:"gap"`
	NoOp  int `json:"no_op" yaml:"no_op"`
}

// Count classifies the block arithmetic. A GAP unit dispositioned
// not-conceptual/no-op lands in NoOp, so the identity closes.
func Count(units []Unit) Counts {
	c := Counts{}
	for _, u := range units {
		if u.Kind == UnitContainer {
			continue
		}
		c.Units++
		switch u.State {
		case StateOK:
			c.OK++
		case StatePart:
			c.Part++
		default:
			switch u.Disposition {
			case DispositionNotConceptual, DispositionNoOp:
				c.NoOp++
			default:
				c.Gap++
			}
		}
	}
	return c
}

// BodyLines counts the artifact body lines, excluding a leading frontmatter
// block, the way sdt doc2md reports `lines`.
func BodyLines(source string) int {
	lines := strings.Split(source, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	body := lines[start:]
	for len(body) > 0 && strings.TrimSpace(body[0]) == "" {
		body = body[1:]
	}
	if n := len(body); n > 0 && body[n-1] == "" {
		body = body[:n-1]
	}
	return len(body)
}

func isFenceDelimiter(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// fenceStateAt reports whether the line at 1-based n is inside a fenced code
// block, so a unit that starts before a fence is measured correctly.
func fenceStateAt(lines []string, n int) bool {
	in := false
	for i := 0; i < n-1 && i < len(lines); i++ {
		if isFenceDelimiter(strings.TrimSpace(lines[i])) {
			in = !in
		}
	}
	return in
}
