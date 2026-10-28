package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ── lesson body checks ────────────────────────────────────────────────────────
//
// A lesson is the teaching composition over one concept page (instructions/
// lesson.md). Its shape is mechanical — four sections in a fixed order and one
// item table with fixed columns — so the shape is a lint, not a prose rule. What
// a check cannot see is whether an expected answer is right: that rests on the
// claim the row names, and on the verification pass that stamped the page.

// ctxLessonSections is the ordered section list a lesson body carries, and no
// other: this slice is the contract.
var ctxLessonSections = []string{"Prerequisites", "Worked example", "Assess", "Transfer"}

// ctxLessonItemColumns is the exact `## Assess` table header, in order.
var ctxLessonItemColumns = []string{"id", "objective", "type", "prompt", "expected", "rubric", "distractor", ctxFrontmatterClaim}

// ctxFrontmatterClaim is the item column naming the claim an expected answer
// rests on; the header and the row reader spell it once.
const ctxFrontmatterClaim = "claim"

// ctxLessonItemTypes is the closed item vocabulary; the order is the difficulty
// ladder and is not restated as a number anywhere.
var ctxLessonItemTypes = []string{"recall", "diagnose", "apply", ctxLessonItemTransfer}

// ctxLessonItemTransfer is the item type every lesson carries at least one of:
// the same idea asked in a case the page did not cover.
const ctxLessonItemTransfer = "transfer"

// ctxLessonClaimRefRegexp is the claim reference grammar an item row carries:
// a page id, then the anchor the page stamps on the claim.
var ctxLessonClaimRefRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*#claim-[0-9]+$`)

// lintLesson validates a lesson document's body: the four sections in order, and
// an item table whose every row is addressable. An empty body is the template's
// scaffold and is not a finding beyond a missing item.
func lintLesson(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeLesson {
		return nil
	}
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}

	body := documentBody(content)
	headings, sections := markdownSections(body)
	issues = append(issues, lintLessonSections(path, headings)...)

	assess, ok := sections["Assess"]
	if !ok {
		return issues
	}
	rows := markdownTableRows(assess)
	if len(rows) == 0 {
		add(ctxLintWarning, "lesson has no `## Assess` table (an item per row, columns "+strings.Join(ctxLessonItemColumns, " | ")+")")
		return issues
	}
	if got := rows[0]; !sameCells(got, ctxLessonItemColumns) {
		add(ctxLintWarning, fmt.Sprintf("item table header is `%s`, want `%s`", strings.Join(got, " | "), strings.Join(ctxLessonItemColumns, " | ")))
	}
	items := rows[1:]
	if len(items) == 0 {
		add(ctxLintWarning, "item table has a header and no rows")
		return issues
	}
	return append(issues, lintLessonItems(path, items)...)
}

// lintLessonSections checks the closed, ordered section list.
func lintLessonSections(path string, headings []string) []ctxLintIssue {
	var issues []ctxLintIssue
	for i, want := range ctxLessonSections {
		switch {
		case i >= len(headings):
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("lesson section `## %s` is missing", want)})
		case headings[i] != want:
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("lesson section %d is `## %s`, want `## %s` (the four sections are ordered)", i+1, headings[i], want)})
		}
	}
	for _, got := range headings[min(len(headings), len(ctxLessonSections)):] {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("unexpected lesson section `## %s` (a lesson carries %s and no more)", got, strings.Join(ctxLessonSections, ", "))})
	}
	return issues
}

// lintLessonItems checks every item row: addressable fields, a claim the page can
// resolve, and at least one `transfer` item.
func lintLessonItems(path string, items [][]string) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}
	seenID := map[string]bool{}
	hasTransfer := false
	for i, cells := range items {
		row := i + 1
		if len(cells) != len(ctxLessonItemColumns) {
			add(ctxLintWarning, fmt.Sprintf("item row %d has %d cell(s), want %d", row, len(cells), len(ctxLessonItemColumns)))
			continue
		}
		it := lessonItem{
			id: cells[0], objective: cells[1], typ: cells[2], prompt: cells[3],
			expected: cells[4], rubric: cells[5], distractor: cells[6], claim: cells[7],
		}
		issues = append(issues, lintLessonItem(path, it, seenID)...)
		if it.typ == ctxLessonItemTransfer {
			hasTransfer = true
		}
	}
	if !hasTransfer {
		add(ctxLintSuggestion, "no `transfer` item: the lesson asks nothing the page did not already answer")
	}
	return issues
}

// lessonItem is one row of the `## Assess` table, named once so the checks read.
type lessonItem struct {
	id, objective, typ, prompt, expected, rubric, distractor, claim string
}

// lintLessonItem checks one item: a unique id, an objective index, a known type,
// a non-empty answer / rubric / distractor, and a resolvable claim reference.
func lintLessonItem(path string, it lessonItem, seenID map[string]bool) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}
	if it.id == "" {
		add(fmt.Sprintf("item row for claim %q has no id", it.claim))
	} else if seenID[it.id] {
		add(fmt.Sprintf("item id %q appears twice in this lesson", it.id))
	}
	seenID[it.id] = true
	if n, err := strconv.Atoi(it.objective); err != nil || n < 1 {
		add(fmt.Sprintf("item %q objective %q is not a 1-based index into the page's `learning.objectives`", it.id, it.objective))
	}
	if !containsItem(ctxLessonItemTypes, it.typ) {
		add(fmt.Sprintf("item %q type %q is not one of %s", it.id, it.typ, strings.Join(ctxLessonItemTypes, ", ")))
	}
	if strings.TrimSpace(it.prompt) == "" {
		add(fmt.Sprintf("item %q has no prompt", it.id))
	}
	if strings.TrimSpace(it.expected) == "" {
		add(fmt.Sprintf("item %q has no expected answer", it.id))
	}
	if strings.TrimSpace(it.rubric) == "" {
		add(fmt.Sprintf("item %q has no rubric", it.id))
	}
	if strings.TrimSpace(it.distractor) == "" {
		add(fmt.Sprintf("item %q has no distractor (a wrong model, not a random option)", it.id))
	}
	if !ctxLessonClaimRefRegexp.MatchString(it.claim) {
		add(fmt.Sprintf("item %q claim %q is not `<page-id>#claim-N`", it.id, it.claim))
	}
	return issues
}

// documentBody strips a leading frontmatter block and returns the body; a document
// without one is already its own body.
func documentBody(content string) string {
	if strings.HasPrefix(content, "---\n") {
		return string(frontmatterBody([]byte(content)))
	}
	return content
}

// markdownSections returns the `##` headings of a body in order, with each
// heading's text mapped to the section body.
func markdownSections(body string) ([]string, map[string]string) {
	var order []string
	bodies := map[string]string{}
	var current string
	var buf []string
	flush := func() {
		if current != "" {
			bodies[current] = strings.Join(buf, "\n")
		}
		buf = buf[:0]
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			order = append(order, current)
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flush()
			current = ""
			continue
		}
		if current != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return order, bodies
}

// markdownTableRows returns the pipe rows of a section, each split into trimmed
// cells. A separator row (`|---|---|`) is dropped, so row 0 is the header.
func markdownTableRows(section string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := lessonTableCells(line)
		if len(cells) > 0 && strings.Trim(cells[0], "-: ") == "" {
			continue // separator
		}
		rows = append(rows, cells)
	}
	return rows
}

// lessonTableCells splits one pipe row into trimmed cells, dropping the empty cells
// the leading and trailing pipes produce.
func lessonTableCells(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func sameCells(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(strings.TrimSpace(a[i]), b[i]) {
			return false
		}
	}
	return true
}

func containsItem(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
