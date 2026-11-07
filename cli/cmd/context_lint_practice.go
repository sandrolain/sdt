package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── practice note checks ──────────────────────────────────────────────────────
//
// A practice note carries the cases that make a concept concrete: it is not
// knowledge, so it has no claims, and the fact check never reads it. What the
// lint can see is its shape and its pointers: a note names at least one concept
// the corpus holds, the span it was transformed from, how it was transformed,
// and at least one case. A draft may still be empty; an active note may not
// (instructions/practice.md).

const ctxPracticeSection = "Cases"

// practiceConceptRe is the closed shape of a concept id: a corpus id, never a
// path. A concept outside it is reported, not joined into a filesystem path.
var practiceConceptRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]*$`)

// lintPractice validates a practice note's pointers and body.
func lintPractice(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypePractice {
		return nil
	}
	var issues []ctxLintIssue
	add := func(msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}

	body := documentBody(content)
	_, bodies := markdownSections(body)
	cases, ok := bodies[ctxPracticeSection]
	if !ok {
		add("a practice note carries no `## " + ctxPracticeSection + "` section")
	} else if countSubsections(cases) == 0 {
		add("a practice note carries no case (`### <case title>` under `## Cases`)")
	}

	if parseFrontmatterField(content, ctxMapStatus) != ctxWikiStatusActive {
		return issues
	}
	concepts := contextwiki.FrontmatterList(content, "concepts")
	if len(concepts) == 0 {
		add("an active practice note names no concept (`concepts`)")
	}
	for _, concept := range concepts {
		if !practiceConceptRe.MatchString(concept) || strings.Contains(concept, "..") {
			add(fmt.Sprintf("concept %q is not a corpus id", concept))
			continue
		}
		page := filepath.Join("context", "wiki", filepath.FromSlash(concept)+sdtMarkdownExt)
		if _, err := os.Stat(page); err != nil { //#nosec G703 -- concept matched against the corpus-id shape above
			add(fmt.Sprintf("concept %q has no page at %s", concept, filepath.ToSlash(page)))
		}
	}
	if strings.TrimSpace(contextwiki.FrontmatterField(content, "source_span")) == "" {
		add("an active practice note carries no `source_span`")
	}
	if strings.TrimSpace(contextwiki.FrontmatterField(content, "transformation")) == "" {
		add("an active practice note carries no `transformation` note")
	}
	return issues
}

// countSubsections counts the H3 headings a section body carries: one case is one
// `### title`.
func countSubsections(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "### ") {
			n++
		}
	}
	return n
}
