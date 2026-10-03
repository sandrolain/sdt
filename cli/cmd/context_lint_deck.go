package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Advisory structural checks for `.slide.md` decks. The rules mirror the
// dialect contract in
// context/instructions/slides-marp.md — change the two together. Findings are
// WARNING (content the engine drops or ignores: a mis-split separator, an
// unknown directive, an unresolved asset) or SUGGESTION (budgets), never
// CRITICAL, so a deck never hard-fails the lint. The checks are pure text: the
// lint path stays independent of the SPA and of `marp-core`.

const (
	// ctxDeckSlideBudget is the soft per-deck slide ceiling (one idea per
	// slide; a deck beyond it is a split signal).
	ctxDeckSlideBudget = 30
	// ctxDeckSlideMin is the soft per-deck slide floor: a one-slide deck is
	// usually a document, not a presentation.
	ctxDeckSlideMin = 2
	// ctxDeckDirectivesPerSlide is the soft ceiling of directive comments on
	// one slide.
	ctxDeckDirectivesPerSlide = 6
)

// ctxDeckKnownDirectives is the built-in Marpit/Marp Core directive set
// (context/instructions/slides-marp.md, `## Directives`). A `_`-prefixed spot
// directive is the same name; an unknown name is silently ignored by the
// engine.
var ctxDeckKnownDirectives = map[string]bool{
	"theme": true, "style": true, "lang": true, "headingDivider": true,
	"size": true, "paginate": true, "header": true, "footer": true,
	"class": true, "backgroundColor": true, "backgroundImage": true,
	"backgroundPosition": true, "backgroundRepeat": true,
	"backgroundSize": true, "color": true,
}

// ctxDeckSeparator is a Marp slide separator ruler.
const ctxDeckSeparator = "---"

// ctxDeckDirectiveRegexp matches an HTML-comment directive body:
// `<!-- key: value -->`. A comment that is not a directive is a speaker note.
var ctxDeckDirectiveRegexp = regexp.MustCompile(`<!--\s*_?([A-Za-z][A-Za-z0-9_-]*)\s*:`)

// ctxDeckImageRegexp matches a markdown image destination `![…](path)`.
var ctxDeckImageRegexp = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]+)`)

// lintDeck runs the advisory deck checks over a `.slide.md` document. content is
// the full file; body is the markdown after the frontmatter.
func lintDeck(path, content string, body []byte) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}
	slides := ctxDeckSplit(string(body))
	ctxDeckLintSeparators(string(body), add)
	ctxDeckLintDirectives(content, add)
	ctxDeckLintAssets(path, string(body), add)
	ctxDeckLintBudgets(slides, add)
	return issues
}

// ctxDeckSplit splits deck markdown into slides on a `---` ruler that is
// surrounded by blank lines (the portable, unambiguous form the contract
// recommends). The frontmatter is already removed by the caller.
func ctxDeckSplit(body string) []string {
	lines := strings.Split(body, "\n")
	var slides []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			slides = append(slides, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == ctxDeckSeparator {
			beforeBlank := i == 0 || strings.TrimSpace(lines[i-1]) == ""
			afterBlank := i == len(lines)-1 || strings.TrimSpace(lines[i+1]) == ""
			if beforeBlank || afterBlank {
				flush()
				continue
			}
		}
		cur = append(cur, line)
	}
	flush()
	return slides
}

// ctxDeckLintSeparators flags a `---` ruler with no blank line before it: the
// silent mis-split trap, where CommonMark reads the previous line plus `---` as
// a setext heading and merges the slides.
func ctxDeckLintSeparators(body string, add ctxMapAdd) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != ctxDeckSeparator {
			continue
		}
		if i == 0 {
			continue
		}
		if strings.TrimSpace(lines[i-1]) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i-1]), "#") {
			continue
		}
		add(ctxLintWarning, fmt.Sprintf("slide separator `---` at line %d has no blank line before it; the previous line plus the ruler may parse as a setext heading and merge the slides", i+1))
	}
}

// ctxDeckLintDirectives flags a directive comment (or a frontmatter directive
// key) whose name is not a built-in: the engine ignores it silently.
func ctxDeckLintDirectives(content string, add ctxMapAdd) {
	for _, m := range ctxDeckDirectiveRegexp.FindAllStringSubmatch(content, -1) {
		name := m[1]
		if !ctxDeckKnownDirectives[name] {
			add(ctxLintWarning, fmt.Sprintf("unknown directive %q (silently ignored; a typo or a name the engine does not know)", name))
		}
	}
}

// ctxDeckLintAssets flags a relative image path that does not resolve on disk.
func ctxDeckLintAssets(path, body string, add ctxMapAdd) {
	dir := filepath.Dir(path)
	for _, m := range ctxDeckImageRegexp.FindAllStringSubmatch(body, -1) {
		dest := strings.Trim(m[1], `"'`)
		if dest == "" || strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "data:") {
			continue
		}
		abs := filepath.Join(dir, dest)
		if _, err := os.Stat(abs); err != nil { //#nosec G703 -- missing file is the finding
			add(ctxLintWarning, "relative asset path does not resolve: "+dest)
		}
	}
}

// ctxDeckLintBudgets emits SUGGESTIONs for a deck outside the soft slide
// bounds.
func ctxDeckLintBudgets(slides []string, add ctxMapAdd) {
	n := len(slides)
	if n < ctxDeckSlideMin {
		add(ctxLintSuggestion, fmt.Sprintf("deck has %d slide(s); a deck below %d is usually a document, not a presentation", n, ctxDeckSlideMin))
		return
	}
	if n > ctxDeckSlideBudget {
		add(ctxLintSuggestion, fmt.Sprintf("deck has %d slides; the default ceiling is %d (one idea per slide)", n, ctxDeckSlideBudget))
	}
	for i, s := range slides {
		if c := len(ctxDeckDirectiveRegexp.FindAllString(s, -1)); c > ctxDeckDirectivesPerSlide {
			add(ctxLintSuggestion, fmt.Sprintf("slide %d carries %d directives; the default ceiling is %d", i+1, c, ctxDeckDirectivesPerSlide))
		}
	}
}
