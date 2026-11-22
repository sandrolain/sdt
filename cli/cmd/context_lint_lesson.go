package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sandrolain/sdt/internal/learn"
)

// ── lesson body checks ────────────────────────────────────────────────────────
//
// A lesson is the teaching composition over one concept page (instructions/
// lesson.md). Its shape is mechanical — four sections in a fixed order and one
// item table with fixed columns — so the shape is a lint, not a prose rule. What
// a check cannot see is whether an expected answer is right: that rests on the
// claim the row names, and on the verification pass that stamped the page.

// ctxLessonSections is the ordered section list a lesson body carries, and no
// other: this slice is the contract. The items live in the concept's assessment
// document, so `## Assess` is no longer a lesson section.
var ctxLessonSections = []string{"Prerequisites", "Worked example", "Transfer"}

// ctxAssessmentSections is the ordered section list an assessment body carries:
// one section, the item table.
var ctxAssessmentSections = []string{learnAssessmentSection}

// ctxLessonItemColumns is the exact `## Assess` table header, in order.
var ctxLessonItemColumns = []string{"id", "objective", "type", "prompt", "expected", "rubric", "distractor", ctxFrontmatterClaim}

// ctxFrontmatterClaim is the item column naming the claim an expected answer
// rests on; the header and the row reader spell it once.
const ctxFrontmatterClaim = "claim"

// ctxLessonItemTypes is the closed item vocabulary; the order is the difficulty
// ladder and is not restated as a number anywhere.
var ctxLessonItemTypes = []string{learn.ItemTypeRecall, learn.ItemTypeDiagnose, learn.ItemTypeApply, ctxLessonItemTransfer}

// ctxLessonItemTransfer is the item type every lesson carries at least one of:
// the same idea asked in a case the page did not cover.
const ctxLessonItemTransfer = learn.ItemTypeTransfer

// ctxLessonClaimRefRegexp is the claim reference grammar an item row carries:
// a page id, then the anchor the page stamps on the claim.
var ctxLessonClaimRefRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*#(?:claim-[0-9]+|c-[a-z0-9]{8})$`)

var ctxChoiceOptionIDRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// lintLesson validates a lesson document's body: the three sections in order, no
// item table of its own. An empty body is the template's scaffold and is not a
// finding beyond a missing section.
func lintLesson(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeLesson {
		return nil
	}
	var issues []ctxLintIssue

	body := documentBody(content)
	headings, _ := markdownSections(body)
	return append(issues, lintLessonSections(path, headings)...)
}

// lintAssessment validates an assessment document's body: the `## Items` section
// and an item table whose every row is addressable and whose key is frozen. The
// checks that only need the table are the lesson's own, because one table format
// serves both documents.
func lintAssessment(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeAssessment {
		return nil
	}
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}

	body := documentBody(content)
	headings, sections := markdownSections(body)
	issues = append(issues, lintSections(path, headings, ctxAssessmentSections, "assessment")...)

	items, ok := sections[learnAssessmentSection]
	if !ok {
		return issues
	}
	rows := markdownTableRows(items)
	if len(rows) == 0 {
		add(ctxLintWarning, "assessment has no item table (an item per row, columns "+strings.Join(ctxLessonItemColumns, " | ")+")")
		return issues
	}
	issues = append(issues, lintLessonHeader(path, rows[0])...)
	index := lessonColumnIndex(rows[0])
	if len(rows) < 2 {
		add(ctxLintWarning, "item table has a header and no rows")
		return issues
	}
	issues = append(issues, lintLessonItems(path, index, rows[1:])...)
	return append(issues, lintAssessmentKeys(path, index, rows[1:])...)
}

// lintAssessmentKeys checks the half of an item the lesson's lint cannot see: the
// grading rule it names, the day its key was frozen, a distractor that is not the
// answer, and the freeze digest that makes the freeze mechanical. A row whose
// key_hash disagrees with its cells is the defect the column exists for: the key
// moved after it was signed.
func lintAssessmentKeys(path string, index map[string]int, items [][]string) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}
	for i, cells := range items {
		row := i + 1
		id := learnLessonCell(cells, index, "id")
		if id == "" {
			id = fmt.Sprintf("row %d", row)
		}
		grading := learnLessonDefault(learnLessonCell(cells, index, "grading"), learnGradingAuto)
		if !containsItem(learnLessonGradingRules, grading) {
			add(fmt.Sprintf("item %q grading %q is not one of %s", id, grading, strings.Join(learnLessonGradingRules, ", ")))
		}
		if _, declared := index["frozen"]; declared && strings.TrimSpace(learnLessonCell(cells, index, "frozen")) == "" {
			add(fmt.Sprintf("item %q has no frozen date: a key is frozen on the day it is written", id))
		}
		expected := learnLessonCell(cells, index, "expected")
		distractor := learnLessonCell(cells, index, "distractor")
		if expected != "" && strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(distractor)) {
			add(fmt.Sprintf("item %q uses its expected answer as the distractor", id))
		}
		interaction := learnLessonCell(cells, index, "interaction")
		options := learnLessonCell(cells, index, "options")
		scoring := learnLessonCell(cells, index, "scoring")
		item := learnLessonItem{
			Expected:    expected,
			Rubric:      learnLessonCell(cells, index, "rubric"),
			Grading:     grading,
			Distractor:  distractor,
			Interaction: interaction,
			Options:     options,
			Scoring:     scoring,
		}
		if item.Interaction != "" {
			item.Scoring = learnLessonDefault(item.Scoring, learn.ScoringExactSet)
		}
		computed := learnItemKeyDigest(item)
		switch stored := learnLessonCell(cells, index, "key_hash"); {
		case stored == "":
			add(fmt.Sprintf("item %q carries no key_hash: its key is not frozen (the row now hashes to %s)", id, computed))
		case stored != computed:
			add(fmt.Sprintf("item %q key_hash %s disagrees with its row (%s): the key moved after it was frozen", id, stored, computed))
		}
		issues = append(issues, lintChoiceFields(path, id, interaction, options, expected, scoring, learnLessonCell(cells, index, "explanation_prompt"))...)
		if interaction != "" && grading != learn.RuleAuto && grading != learn.RuleExact {
			add(fmt.Sprintf("item %q choice interaction must use auto or exact grading; QTI scoring is declared in scoring", id))
		}
	}
	return issues
}

func lintChoiceFields(path, id, interaction, options, expected, scoring, explanationPrompt string) []ctxLintIssue {
	var issues []ctxLintIssue
	if interaction == "" {
		if options != "" || scoring != "" || explanationPrompt != "" {
			issues = append(issues, choiceIssue(path, id, "has choice metadata without an interaction"))
		}
		return issues
	}
	if !knownChoiceInteraction(interaction) {
		issues = append(issues, choiceIssue(path, id, fmt.Sprintf("interaction %q is unsupported", interaction)))
		return issues
	}
	choices, optionIssues, ok := lintChoiceOptions(path, id, options)
	issues = append(issues, optionIssues...)
	if !ok {
		return issues
	}
	correct, keyIssues, ok := lintChoiceKey(path, id, expected, choices)
	issues = append(issues, keyIssues...)
	if !ok {
		return issues
	}
	scoring = learnLessonDefault(scoring, learn.ScoringExactSet)
	issues = append(issues, lintChoiceScoring(path, id, interaction, scoring)...)
	issues = append(issues, lintChoiceCardinality(path, id, interaction, len(choices), len(correct))...)
	issues = append(issues, lintChoiceExplanation(path, id, interaction, explanationPrompt)...)
	return issues
}

func choiceIssue(path, id, message string) ctxLintIssue {
	return ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("item %q %s", id, message)}
}

func knownChoiceInteraction(interaction string) bool {
	return containsItem([]string{learn.InteractionSingleChoice, learn.InteractionMultipleChoice,
		learn.InteractionTrueFalse, learn.InteractionChoiceExplanation}, interaction)
}

func lintChoiceOptions(path, id, raw string) ([]learnChoiceOption, []ctxLintIssue, bool) {
	var choices []learnChoiceOption
	if err := json.Unmarshal([]byte(raw), &choices); err != nil || len(choices) < 2 {
		return nil, []ctxLintIssue{choiceIssue(path, id, "options must be a JSON array of at least two {id,text} objects")}, false
	}
	var issues []ctxLintIssue
	ids := map[string]bool{}
	for _, choice := range choices {
		if !ctxChoiceOptionIDRegexp.MatchString(choice.ID) || strings.TrimSpace(choice.Text) == "" {
			issues = append(issues, choiceIssue(path, id, "has an option with an invalid id or empty text"))
		}
		if ids[choice.ID] {
			issues = append(issues, choiceIssue(path, id, fmt.Sprintf("repeats option id %q", choice.ID)))
		}
		ids[choice.ID] = true
	}
	return choices, issues, true
}

func lintChoiceKey(path, id, raw string, choices []learnChoiceOption) ([]string, []ctxLintIssue, bool) {
	var correct []string
	if err := json.Unmarshal([]byte(raw), &correct); err != nil || len(correct) == 0 {
		return nil, []ctxLintIssue{choiceIssue(path, id, "expected must be a JSON array of correct option ids")}, false
	}
	known := make(map[string]bool, len(choices))
	for _, choice := range choices {
		known[choice.ID] = true
	}
	var issues []ctxLintIssue
	seen := map[string]bool{}
	for _, answer := range correct {
		if seen[answer] {
			issues = append(issues, choiceIssue(path, id, fmt.Sprintf("repeats expected option id %q", answer)))
		}
		if !known[answer] {
			issues = append(issues, choiceIssue(path, id, fmt.Sprintf("expected option id %q is not present in options", answer)))
		}
		seen[answer] = true
	}
	return correct, issues, true
}

func lintChoiceCardinality(path, id, interaction string, optionCount, correctCount int) []ctxLintIssue {
	var issues []ctxLintIssue
	if (interaction == learn.InteractionSingleChoice || interaction == learn.InteractionTrueFalse || interaction == learn.InteractionChoiceExplanation) && correctCount != 1 {
		issues = append(issues, choiceIssue(path, id, "requires exactly one expected option id"))
	}
	if interaction == learn.InteractionTrueFalse && optionCount != 2 {
		issues = append(issues, choiceIssue(path, id, "true-false requires exactly two options"))
	}
	return issues
}

func lintChoiceScoring(path, id, interaction, scoring string) []ctxLintIssue {
	var issues []ctxLintIssue
	if scoring != learn.ScoringExactSet && scoring != learn.ScoringPartialCredit {
		issues = append(issues, choiceIssue(path, id, fmt.Sprintf("scoring %q must be exact-set or partial-credit", scoring)))
	}
	if scoring == learn.ScoringPartialCredit && interaction != learn.InteractionMultipleChoice {
		issues = append(issues, choiceIssue(path, id, "partial-credit is supported only for multiple-choice"))
	}
	return issues
}

func lintChoiceExplanation(path, id, interaction, prompt string) []ctxLintIssue {
	var issues []ctxLintIssue
	if interaction == learn.InteractionChoiceExplanation && strings.TrimSpace(prompt) == "" {
		issues = append(issues, choiceIssue(path, id, "choice-explanation requires explanation_prompt"))
	}
	if interaction != learn.InteractionChoiceExplanation && prompt != "" {
		issues = append(issues, choiceIssue(path, id, "explanation_prompt is only valid for choice-explanation"))
	}
	return issues
}

// lintLessonHeader checks the item table header: the eight base columns in order,
// then zero or more of the optional trailing columns, also in order. Any other
// column is refused, because the parser would silently ignore it.
func lintLessonHeader(path string, header []string) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}
	for i, want := range ctxLessonItemColumns {
		switch {
		case i >= len(header):
			add(fmt.Sprintf("item table header is missing column %q", want))
		case !strings.EqualFold(strings.TrimSpace(header[i]), want):
			add(fmt.Sprintf("item table header column %d is %q, want %q", i+1, header[i], want))
		}
	}
	// The optional columns are a trailing *subsequence*: an item table may carry
	// `hash` without the three that precede it, but may never go back in order.
	seen := 0
	for i := len(ctxLessonItemColumns); i < len(header); i++ {
		cell := strings.ToLower(strings.TrimSpace(header[i]))
		if match := optionalColumnAtOrAfter(cell, seen); match >= 0 {
			seen = match + 1
			continue
		}
		add(fmt.Sprintf("item table column %q is not an optional trailing column (%s, in that order)",
			header[i], strings.Join(learnLessonOptionalColumns, ", ")))
	}
	return issues
}

// optionalColumnAtOrAfter returns the index of an optional column at or after
// `from`, or -1 when the cell is not one of them.
func optionalColumnAtOrAfter(cell string, from int) int {
	for j := from; j < len(learnLessonOptionalColumns); j++ {
		if cell == learnLessonOptionalColumns[j] {
			return j
		}
	}
	return -1
}

// lessonColumnIndex maps every header cell to its column, so a row is read by
// name and an optional column can never shift a base one.
func lessonColumnIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, cell := range header {
		index[strings.ToLower(strings.TrimSpace(cell))] = i
	}
	return index
}

// lintSections checks the closed, ordered section list of one document kind.
func lintSections(path string, headings, want []string, kind string) []ctxLintIssue {
	var issues []ctxLintIssue
	for i, w := range want {
		switch {
		case i >= len(headings):
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s section `## %s` is missing", kind, w)})
		case headings[i] != w:
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("%s section %d is `## %s`, want `## %s` (the sections are ordered)", kind, i+1, headings[i], w)})
		}
	}
	for _, got := range headings[min(len(headings), len(want)):] {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("unexpected %s section `## %s` (a %s carries %s and no more)", kind, got, kind, strings.Join(want, ", "))})
	}
	return issues
}

// lintLessonSections checks the closed, ordered section list of a lesson body.
func lintLessonSections(path string, headings []string) []ctxLintIssue {
	return lintSections(path, headings, ctxLessonSections, "lesson")
}

// lintLessonItems checks every item row: addressable fields, a claim the page can
// resolve, and at least one `transfer` item.
func lintLessonItems(path string, index map[string]int, items [][]string) []ctxLintIssue {
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}
	seenID := map[string]bool{}
	hasTransfer := false
	maxCells := len(ctxLessonItemColumns) + len(learnLessonOptionalColumns)
	for i, cells := range items {
		row := i + 1
		if len(cells) < len(ctxLessonItemColumns) || len(cells) > maxCells {
			add(ctxLintWarning, fmt.Sprintf("item row %d has %d cell(s), want %d to %d",
				row, len(cells), len(ctxLessonItemColumns), maxCells))
			continue
		}
		it := lessonItem{
			id:         learnLessonCell(cells, index, "id"),
			objective:  learnLessonCell(cells, index, "objective"),
			typ:        learnLessonCell(cells, index, "type"),
			prompt:     learnLessonCell(cells, index, "prompt"),
			expected:   learnLessonCell(cells, index, "expected"),
			rubric:     learnLessonCell(cells, index, "rubric"),
			distractor: learnLessonCell(cells, index, "distractor"),
			claim:      learnLessonCell(cells, index, ctxFrontmatterClaim),
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
		add(fmt.Sprintf("item %q claim %q is not `<page-id>#c-<id>` (the positional `#claim-N` is the legacy form)", it.id, it.claim))
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

func containsItem(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
