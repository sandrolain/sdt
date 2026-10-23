package distill

import (
	"regexp"
	"strconv"
	"strings"
)

// citationRe matches the corpus citation form `refs/<path>@<hex>{6,}` with an
// optional `:n` or `:n-m` line range. It is the same shape `sdt context lint`
// verifies, extended here to capture the range.
var citationRe = regexp.MustCompile(`(refs/[A-Za-z0-9_./-]+)@([0-9a-f]{6,})(?::([0-9]+)(?:-([0-9]+))?)?`)

var inlineCodeRe = regexp.MustCompile("`[^`\n]*`")

// Citation is one hash-anchored reference to a source line or range.
type Citation struct {
	Ref   string
	SHA   string
	Line  int // 0 when the citation names no lines
	End   int
	Whole bool
}

// ParseCitations reads the citations in a page body. Fenced code blocks and
// inline code spans are blanked first: an example is not a citation.
func ParseCitations(body string) []Citation {
	body = inlineCodeRe.ReplaceAllString(stripFenced(body), "")
	var out []Citation
	for _, m := range citationRe.FindAllStringSubmatch(body, -1) {
		c := Citation{Ref: m[1], SHA: m[2]}
		if m[3] != "" {
			if n, err := strconv.Atoi(m[3]); err == nil {
				c.Line, c.End = n, n
			}
			if m[4] != "" {
				if n, err := strconv.Atoi(m[4]); err == nil {
					c.End = n
				}
			}
		} else {
			c.Whole = true
		}
		out = append(out, c)
	}
	return out
}

// RangesFor keeps the citations of one artifact and returns their spans. A
// citation without lines cannot be classified and is reported through whole.
func RangesFor(citations []Citation, artifact string) (ranges []Range, whole int) {
	want := normalizeRef(artifact)
	for _, c := range citations {
		if normalizeRef(c.Ref) != want {
			continue
		}
		if c.Whole {
			whole++
			continue
		}
		ranges = append(ranges, Range{Start: c.Line, End: c.End, Ref: c.Ref})
	}
	return ranges, whole
}

// normalizeRef compares refs by their path below refs/.
func normalizeRef(ref string) string {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "context/")
	return strings.TrimPrefix(ref, "refs/")
}

// stripFenced blanks fenced code blocks while keeping line count, so a citation
// inside a code block never reaches the matcher.
func stripFenced(body string) string {
	lines := strings.Split(body, "\n")
	fence := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		switch {
		case fence == "" && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")):
			fence = trim[:3]
			lines[i] = ""
		case fence != "" && strings.HasPrefix(trim, fence):
			fence = ""
			lines[i] = ""
		case fence != "":
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
