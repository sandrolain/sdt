//go:build !wasm

package cmd

import (
	"fmt"
	"os"
	"strings"
)

// The structural deck check: what a render needs from a `.slide.md` deck, read
// offline and in pure Go (decision/0036 keeps the full Marp render in the
// viewer). The normalization is the viewer's own rule —
// `web/src/lib/slidesSource.ts`, `normalizeDeckSource` — re-implemented here as
// a shared contract, pinned against `web/src/lib/slidesSource.test.ts` like the
// ACP wire fixtures.
//
// Marp reads its frontmatter (the `theme:`, `paginate:`, `size:` block) only
// when it is the first thing in the document. A material deck carries its
// machine header comment first (decision/0038) and the viewer drops that header
// before rendering; a deck that needs the drop is render-broken all the same,
// because the deck as authored opens on the header and shows the frontmatter
// keys as slide text. The check reads the deck as authored, not as the viewer
// rescues it.

// deckWhitespace is the `\s` character class the shared rule normalizes over.
const deckWhitespace = "\t\n\f\r "

// normalizeDeckSource mirrors `normalizeDeckSource` in
// web/src/lib/slidesSource.ts: a leading HTML comment immediately followed by a
// `---` frontmatter block is a file header, not slide content, and is dropped
// so the frontmatter starts the document. A leading comment followed by a
// separator is left alone — it is a speaker note on slide 1, ordinary Marp.
func normalizeDeckSource(source string) string {
	rest := strings.TrimLeft(source, deckWhitespace)
	if !strings.HasPrefix(rest, "<!--") {
		return source
	}
	close_ := strings.Index(rest, "-->")
	if close_ < 0 {
		return source
	}
	// Between the comment close and the frontmatter opener every character is
	// whitespace (`\s*\n+` in the TS rule, which is one whitespace run).
	trimmed := strings.TrimLeft(rest[close_+len("-->"):], deckWhitespace)
	if !strings.HasPrefix(trimmed, ctxFrontmatterDelim+"\n") {
		return source
	}
	if body := trimmed[len(ctxFrontmatterDelim)+1:]; body == "" || body[0] == '\n' {
		// `---` followed by a blank line is a slide separator, not a
		// frontmatter block; the leading note stays.
		return source
	}
	return trimmed
}

// deckRenderBroken is the material status a structurally broken deck reports
// instead of `fresh`.
const deckRenderBroken = "render-broken"

// deckStructureIssue reports why a deck cannot render, or "" when it can. The
// check parses the deck the way the viewer renders it — normalizeDeckSource
// first, so the machine header comment (decision/0038) is out of Marp's way —
// and then asks for the structure a render needs:
//
//  1. after normalization the frontmatter must be the first block: Marp reads
//     the `theme:`, `paginate:`, `size:` keys only there, so a deck whose
//     frontmatter is displaced beyond the header (by a separator, by slide
//     text, or absent) renders those keys as slide content;
//  2. the frontmatter block must close, and at least one slide section must
//     follow it;
//  3. no frontmatter key (`theme:`, `paginate:`, `size:`) may appear as slide
//     text.
//
// The full Marp render stays the viewer's job (decision/0036): this is the
// structure a render needs, and a deck that fails it cannot report fresh.
func deckStructureIssue(source string) string {
	lines := strings.Split(normalizeDeckSource(source), "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != ctxFrontmatterDelim {
		return "the frontmatter is not the first block: a render reads the theme, paginate and size keys only there"
	}
	closing := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			closing = i
			break
		}
	}
	if closing < 0 {
		return "the frontmatter block never closes"
	}
	rest := lines[closing+1:]
	if !deckHasSlideContent(rest) {
		return "the frontmatter block is followed by no slide section"
	}
	for _, line := range rest {
		trimmed := strings.TrimSpace(line)
		for _, key := range []string{"theme:", "paginate:", "size:"} {
			if strings.HasPrefix(trimmed, key) {
				return fmt.Sprintf("the frontmatter key %s appears as slide text", key)
			}
		}
	}
	return ""
}

// deckHasSlideContent reports whether any line of the deck after the
// frontmatter block carries content.
func deckHasSlideContent(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// deckRenderIssue runs the structural check over the deck file beside a guide.
// A deck that cannot be read says so: silence is not evidence here either.
func deckRenderIssue(path string) (string, bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- the deck beside the resolved guide
	if err != nil {
		return "cannot read the deck file", false
	}
	return deckStructureIssue(string(data)), true
}
