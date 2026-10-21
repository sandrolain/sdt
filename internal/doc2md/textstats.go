package doc2md

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// sectionHeadingRegexp matches the section-level headings the route doctrine
// counts: markdown levels 2 and 3, the lines that demarcate sections. Level 1
// is the document or chapter title, not a section; an artifact that demarcates
// its sections at another level counts that level in the doctrine's own words —
// the caller that hits such an artifact records it in the receipt.
var sectionHeadingRegexp = regexp.MustCompile(`(?m)^#{2,3} `)

// Measure returns the routing-test counts on a converted artifact's markdown
// body: its section-level headings and its lines. Both are plain text
// measurements, defined once here because the conversion receipt and the corpus
// listing must never disagree. The provenance frontmatter is not part of the
// conversion and is not measured.
func Measure(markdown string) (headings, lines int) {
	headings = len(sectionHeadingRegexp.FindAllString(markdown, -1))
	// Leading blank lines (the separator the provenance renderer leaves after a
	// frontmatter block) are not the artifact's first line.
	body := strings.TrimLeft(strings.TrimSuffix(markdown, "\n"), "\n")
	if body == "" {
		return headings, 0
	}
	return headings, strings.Count(body, "\n") + 1
}

// MeasureArtifact measures a kept artifact on disk: the body after the
// provenance frontmatter. The corpus listing uses it so the counts it reports
// can be refilled for artifacts kept before the reporting existed.
func MeasureArtifact(path string) (headings, lines int, err error) {
	data, err := os.ReadFile(path) //#nosec G304 -- path joined from a corpus scan, not user input
	if err != nil {
		return 0, 0, fmt.Errorf("read %s: %w", path, err)
	}
	headings, lines = Measure(splitBody(string(data)))
	return headings, lines, nil
}
